/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"time"

	customConfigMapv1alpha1 "github.com/gopaddle-io/configurator/apis/configurator.gopaddle.io/v1alpha1"
	appsV1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMapReconciler reconciles a ConfigMap object
type ConfigMapReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

var log = ctrl.Log.WithName("SecretController")

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConfigMap object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var configMaplogname string = req.Namespace + "/" + req.Name
	var configMap corev1.ConfigMap

	//get configMap
	err := r.Get(ctx, req.NamespacedName, &configMap)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Info(configMaplogname + " Unable to get configMap: " + err.Error())
		} else {
			log.Error(err, configMaplogname+" Unable to get configMap")
		}
		return ctrl.Result{}, client.IgnoreNotFound(nil)
	}

	//get currentCCM version from configMap
	currentCCM_Version := configMap.Annotations["currentCustomConfigMapVersion"]
	// check the CCM_version in the configMap if the Version not exist.
	// it create new version of CCM and add annotation to that configMap
	if currentCCM_Version == "" {
		ccm, version := newCustomConfigMap(&configMap)
		er := r.Create(ctx, ccm)
		if er != nil {
			r.EventRecorder.Eventf(&configMap, corev1.EventTypeWarning, "FailedCreateCustomConfigMap", "Error creating CustomConfigMap: %v", er.Error())
			return ctrl.Result{}, er
		}

		//updating configMap with versionInfo
		if len(configMap.Annotations) == 0 {
			annotation := make(map[string]string)
			annotation["currentCustomConfigMapVersion"] = version
			annotation["customConfigMap-name"] = ccm.Name
			annotation["updateMethod"] = "ignoreWhenShared"
			configMap.Annotations = annotation
		} else {
			configMap.Annotations["currentCustomConfigMapVersion"] = version
			configMap.Annotations["customConfigMap-name"] = ccm.Name
			configMap.Annotations["updateMethod"] = "ignoreWhenShared"
		}

		errs := r.Update(ctx, &configMap)
		if errs != nil {
			r.EventRecorder.Eventf(&configMap, corev1.EventTypeWarning, "FailedAddingCustomConfigMapVersion", "Error in adding CustomConfigMap version: %v", errs.Error())
			return ctrl.Result{}, errs
		}
		r.EventRecorder.Eventf(&configMap, corev1.EventTypeNormal, "configMap", "update ccm content %v to configMap %v", ccm.Name, configMap.Name)
	} else {
		// version exist it compare the configMap content with currentCCM
		er := r.UpdateConfigMap(ctx, &configMap)
		if er != nil {
			r.EventRecorder.Eventf(&configMap, corev1.EventTypeNormal, "FailedCreateCustomConfigMapVersion", "Error in creating CustomConfigMap: %v", er.Error())
			return ctrl.Result{}, er
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}

// newCustomConfigMap creates a new customConfigMap for a ConfigMap resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ConfigMap resource that 'owns' it.
func newCustomConfigMap(configmap *corev1.ConfigMap) (*customConfigMapv1alpha1.CustomConfigMap, string) {
	labels := map[string]string{
		"name":    configmap.Name,
		"latest":  "true",
		"current": "true",
	}

	version := RandomSequence(5)
	name := fmt.Sprintf("%s-%s", configmap.Name, version)
	configName := NameValidation(name)
	annotation := map[string]string{
		"customConfigMapVersion": version,
	}
	ccm := &customConfigMapv1alpha1.CustomConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configName,
			Namespace: configmap.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(configmap, corev1.SchemeGroupVersion.WithKind("ConfigMap")),
			},
			Labels:      labels,
			Annotations: annotation,
		},
		Spec: customConfigMapv1alpha1.CustomConfigMapSpec{
			Data:          configmap.Data,
			BinaryData:    configmap.BinaryData,
			ConfigMapName: configmap.Name,
		},
	}
	return ccm, version
}

//Update ConfigMap
func (r *ConfigMapReconciler) UpdateConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	//get CCM latest version
	var configMaplogname string = configMap.Namespace + "/" + configMap.Name
	var ccmList customConfigMapv1alpha1.CustomConfigMapList
	err := r.List(ctx, &ccmList, client.MatchingLabels{"name": configMap.Name, "current": "true"}, client.InNamespace(configMap.Namespace))
	if err != nil {
		log.Error(err, configMaplogname+" Unable to get customConfigMap list")
		return err
	}
	if len(ccmList.Items) == 1 {
		//checking configmap annotation version with currentCCM version
		if ccmList.Items[0].Annotations["customConfigMapVersion"] != configMap.Annotations["currentCustomConfigMapVersion"] {
			//calling copy CCM to CM function
			er := r.CopyCCMToCM(ctx, configMap)
			if er != nil {
				return er
			}

			//update ccm to remove current
			delete(ccmList.Items[0].Labels, "current")
			errs := r.Update(ctx, &ccmList.Items[0])
			if errs != nil {
				r.EventRecorder.Eventf(configMap, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
				return errs
			}
		} else {
			//content of configMap and customConfigMap are not same create newCCM and make that as current
			if reflect.DeepEqual(configMap.Data, ccmList.Items[0].Spec.Data) == false || reflect.DeepEqual(configMap.BinaryData, ccmList.Items[0].Spec.BinaryData) == false {
				errs := r.CreateNewCCM(ctx, configMap, &ccmList.Items[0])
				if errs != nil {
					return errs
				}

			}
		}
	}
	return nil
}

//copyCCMtoCM
func (r *ConfigMapReconciler) CopyCCMToCM(ctx context.Context, configmap *corev1.ConfigMap) error {
	var ccm customConfigMapv1alpha1.CustomConfigMap
	ccmNameNamespace := types.NamespacedName{
		Namespace: configmap.Namespace,
		Name:      configmap.Name + "-" + configmap.Annotations["currentCustomConfigMapVersion"],
	}
	er := r.Get(ctx, ccmNameNamespace, &ccm)
	if er != nil {
		r.EventRecorder.Eventf(configmap, corev1.EventTypeWarning, "FailedGetCustomConfigMap", "Error Getting CustomConfigMap: %v", er.Error())
		return er
	}

	//copying content
	configmap.Data = ccm.Spec.Data
	configmap.BinaryData = ccm.Spec.BinaryData
	//Update configMap content based on configmap version
	err := r.Update(ctx, configmap)
	if err != nil {
		return err
	}

	//update CCM with current
	ccm.Labels["current"] = "true"
	errs := r.Update(ctx, &ccm)
	if errs != nil {
		r.EventRecorder.Eventf(&ccm, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
		return errs
	}

	r.EventRecorder.Eventf(configmap, corev1.EventTypeNormal, "configMap", "update ccm content %v to configMap %v", ccm.Name, configmap.Name)
	return nil
}

func (r *ConfigMapReconciler) CreateNewCCM(ctx context.Context, configMap *corev1.ConfigMap, currentccm *customConfigMapv1alpha1.CustomConfigMap) error {
	//delete current label from previous currentCCM
	delete(currentccm.Labels, "current")
	errs := r.Update(ctx, currentccm)
	if errs != nil {
		r.EventRecorder.Eventf(currentccm, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
		return errs
	}

	//delete latest label from latest ccm
	//getting latest ccm
	latestccm := customConfigMapv1alpha1.CustomConfigMapList{}
	err := r.List(ctx, &latestccm, client.MatchingLabels{"name": configMap.Name, "latest": "true"}, client.InNamespace(configMap.Namespace))
	if err != nil {
		r.EventRecorder.Eventf(&latestccm, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
		return err
	}
	delete(latestccm.Items[0].Labels, "latest")
	errs = r.Update(ctx, &latestccm.Items[0])
	if errs != nil {
		r.EventRecorder.Eventf(&latestccm, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
		//reverting the previous change
		currentccm.Annotations["current"] = "true"
		errs := r.Update(ctx, currentccm)
		if errs != nil {
			r.EventRecorder.Eventf(&latestccm, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
			return errs
		}
		return errs
	}

	ccmNew, version := newCustomConfigMap(configMap)
	er := r.Create(ctx, ccmNew)
	if er != nil {
		r.EventRecorder.Eventf(ccmNew, corev1.EventTypeWarning, "FailedCreateCustomConfigMap", "Error creating CustomConfigMap: %v", er)
		//reverting the previous change
		currentccm.Annotations["current"] = "true"
		errs := r.Update(ctx, currentccm)
		if errs != nil {
			r.EventRecorder.Eventf(currentccm, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
			return errs
		}
		latestccm.Items[0].Annotations["latest"] = "true"
		errs = r.Update(ctx, &latestccm.Items[0])
		if errs != nil {
			r.EventRecorder.Eventf(&latestccm, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
			return errs
		}
		return er
	}

	//update config map with version and ccm name
	configMap.Annotations["currentCustomConfigMapVersion"] = version
	configMap.Annotations["customConfigMap-name"] = ccmNew.Name
	configMap.Annotations["updateMethod"] = "ignoreWhenShared"
	err = r.Update(ctx, configMap)
	if err != nil {
		return err
	}
	r.EventRecorder.Eventf(configMap, corev1.EventTypeNormal, "updateConfigMap", "update ccm version %v and name %v", version, ccmNew.Name)

	//trigger rolling Update for kind=deployment
	if configMap.Annotations["deployments"] != "" {
		annotation := configMap.Annotations["deployments"]
		split := strings.Split(annotation, ",")
		if configMap.Annotations["updateMethod"] == "ignoreWhenShared" {
			if len(split) > 1 {
				klog.Error("can't trigger rolling update updateMethod is ignoreWhenShared")
				return errors.NewBadRequest("can't trigger rolling update updateMethod is ignoreWhenShared")
			} else {
				var deployment appsV1.Deployment
				deployNameNamespace := types.NamespacedName{
					Namespace: configMap.Namespace,
					Name:      split[0],
				}
				err := r.Get(ctx, deployNameNamespace, &deployment)
				if err != nil {
					klog.Error("Failed on getting deployment '%s' Error: %s", split[0], err.Error)
					return err
				}
				//update new version
				deployment.Spec.Template.Annotations["ccm-"+configMap.Name] = version
				err = r.Update(ctx, &deployment)
				if err != nil {
					klog.Error("Failed on updating deployment '%s' Error: %s", split[0], err.Error)
					return err
				}
			}
		}
	}
	if configMap.Annotations["statefulsets"] != "" {
		annotation := configMap.Annotations["statefulsets"]
		split := strings.Split(annotation, ",")
		if configMap.Annotations["updateMethod"] == "ignoreWhenShared" {
			if len(split) > 1 {
				klog.Error("can't trigger rolling update updateMethod is ignoreWhenShared")
				return errors.NewBadRequest("can't trigger rolling update updateMethod is ignoreWhenShared")
			} else {
				var sts appsV1.StatefulSet
				stsNameNamespace := types.NamespacedName{
					Namespace: configMap.Namespace,
					Name:      split[0],
				}

				err := r.Get(ctx, stsNameNamespace, &sts)
				if err != nil {
					klog.Error("Failed on getting statefulset '%s' Error: %s", split[0], err.Error)
					return err
				}
				//update new version
				sts.Spec.Template.Annotations["ccm-"+configMap.Name] = version
				err = r.Update(ctx, &sts)
				if err != nil {
					klog.Error("Failed on updating statefulset '%s' Error: %s", split[0], err.Error)
					return err
				}
			}
		}
	}
	return nil
}

func RandomSequence(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func NameValidation(name string) string {
	reg, err := regexp.Compile("[^a-z0-9-]+")
	if err != nil {
		panic(err)
	}

	return reg.ReplaceAllString(name, "s")
}
