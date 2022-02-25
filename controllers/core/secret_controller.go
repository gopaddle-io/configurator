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
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	customSecretv1alpha1 "github.com/gopaddle-io/configurator/apis/configurator.gopaddle.io/v1alpha1"
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

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

var slog = ctrl.Log.WithName("SecretController")

//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Secret object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var secretlogname string = req.Namespace + "/" + req.Name
	var secret corev1.Secret

	//get Secret
	err := r.Get(ctx, req.NamespacedName, &secret)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			slog.Info(secretlogname + " Unable to get secret: " + err.Error())
		} else {
			slog.Error(err, secretlogname+" Unable to get secret")
		}
		return ctrl.Result{}, client.IgnoreNotFound(nil)
	}

	//get currentCCM version from configMap
	currentCS_Version := secret.Annotations["currentCustomSecretVersion"]
	// check the CCM_version in the configMap if the Version not exist.
	// it create new version of CCM and add annotation to that configMap
	if currentCS_Version == "" {
		var csList customSecretv1alpha1.CustomSecretList
		err := r.List(ctx, &csList, client.MatchingLabels{"name": secret.Name, "current": "true"}, client.InNamespace(secret.Namespace))
		if err != nil {
			log.Error(err, secretlogname+" Unable to get customSecret list")
			return ctrl.Result{}, err
		}
		if len(csList.Items) == 0 {
			cs, version := newCustomSecret(&secret)
			er := r.Create(ctx, cs)
			if er != nil {
				r.EventRecorder.Eventf(&secret, corev1.EventTypeWarning, "FailedCreateCustomSecret", "Error creating CustomSecret: %v", er.Error())
				return ctrl.Result{}, er
			}

			//updating configMap with versionInfo
			if len(secret.Annotations) == 0 {
				annotation := make(map[string]string)
				annotation["currentCustomSecretVersion"] = version
				annotation["customSecret-name"] = cs.Name
				annotation["updateMethod"] = "ignoreWhenShared"
				secret.Annotations = annotation
			} else {
				secret.Annotations["currentCustomSecretVersion"] = version
				secret.Annotations["customSecret-name"] = cs.Name
				secret.Annotations["updateMethod"] = "ignoreWhenShared"
			}
			errs := r.Update(ctx, &secret)
			if errs != nil {
				r.EventRecorder.Eventf(&secret, corev1.EventTypeWarning, "FailedAddingCustomSecretVersion", "Error in adding CustomSecret version: %v", errs.Error())
				return ctrl.Result{}, errs
			}
			r.EventRecorder.Eventf(&secret, corev1.EventTypeNormal, "configMap", "update cs content %v to secret %v", secret.Name, cs.Name)
		} else {
			if len(csList.Items) == 1 {
				secretAnnotation := make(map[string]string)
				data, _ := json.Marshal(secret.ObjectMeta.Annotations)
				_ = json.Unmarshal(data, &secretAnnotation)
				delete(secretAnnotation, "currentCustomSecretVersion")
				delete(secretAnnotation, "customSecret-name")
				delete(secretAnnotation, "updateMethod")
				delete(secretAnnotation, "deployments")
				delete(secretAnnotation, "statefulsets")
				//content of configMap and customSecret are not same create newCS and make that as current
				if len(csList.Items[0].Spec.SecretAnnotations) != 0 {
					if reflect.DeepEqual(secret.Data, csList.Items[0].Spec.Data) == false || secret.Type != csList.Items[0].Spec.Type || reflect.DeepEqual(secretAnnotation, csList.Items[0].Spec.SecretAnnotations) == false {
						errs := r.CreateNewCS(ctx, &secret, &csList.Items[0])
						if errs != nil {
							return ctrl.Result{}, errs
						}

					} else {
						//updating configMap with versionInfo
						if len(secret.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["currentCustomSecretVersion"] = csList.Items[0].Annotations["customSecretVersion"]
							annotation["customSecret-name"] = csList.Items[0].Name
							annotation["updateMethod"] = "ignoreWhenShared"
							secret.Annotations = annotation
						} else {
							secret.Annotations["currentCustomSecretVersion"] = csList.Items[0].Annotations["customSecretVersion"]
							secret.Annotations["customSecret-name"] = csList.Items[0].Name
							secret.Annotations["updateMethod"] = "ignoreWhenShared"
						}
						errs := r.Update(ctx, &secret)
						if errs != nil {
							r.EventRecorder.Eventf(&secret, corev1.EventTypeWarning, "FailedAddingCustomSecretVersion", "Error in adding CustomSecret version: %v", errs.Error())
							return ctrl.Result{}, errs
						}
						r.EventRecorder.Eventf(&secret, corev1.EventTypeNormal, "secret", "update cs content %v to secret %v", secret.Name, csList.Items[0].Name)
					}
				} else {
					if reflect.DeepEqual(secret.Data, csList.Items[0].Spec.Data) == false || secret.Type != csList.Items[0].Spec.Type {
						errs := r.CreateNewCS(ctx, &secret, &csList.Items[0])
						if errs != nil {
							return ctrl.Result{}, errs
						}

					} else {
						//updating configMap with versionInfo
						if len(secret.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["currentCustomSecretVersion"] = csList.Items[0].Annotations["customSecretVersion"]
							annotation["customSecret-name"] = csList.Items[0].Name
							annotation["updateMethod"] = "ignoreWhenShared"
							secret.Annotations = annotation
						} else {
							secret.Annotations["currentCustomSecretVersion"] = csList.Items[0].Annotations["customSecretVersion"]
							secret.Annotations["customSecret-name"] = csList.Items[0].Name
							secret.Annotations["updateMethod"] = "ignoreWhenShared"
						}
						errs := r.Update(ctx, &secret)
						if errs != nil {
							r.EventRecorder.Eventf(&secret, corev1.EventTypeWarning, "FailedAddingCustomSecretVersion", "Error in adding CustomSecret version: %v", errs.Error())
							return ctrl.Result{}, errs
						}
						r.EventRecorder.Eventf(&secret, corev1.EventTypeNormal, "secret", "update cs content %v to secret %v", secret.Name, csList.Items[0].Name)
					}
				}
			}
		}

	} else {
		// version exist it compare the configMap content with currentCCM
		er := r.UpdateSecret(ctx, &secret)
		if er != nil {
			r.EventRecorder.Eventf(&secret, corev1.EventTypeNormal, "FailedCreateCustomSecretVersion", "Error in creating CustomSecret: %v", er.Error())
			return ctrl.Result{}, er
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}

// newSecret creates a new Secret for a CustomSecret resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the CustomSecret resource that 'owns' it.
func newCustomSecret(secret *corev1.Secret) (*customSecretv1alpha1.CustomSecret, string) {
	labels := map[string]string{
		"name":    secret.Name,
		"latest":  "true",
		"current": "true",
	}
	version := RandomSequence(5)
	name := fmt.Sprintf("%s-%s", secret.Name, version)
	secretName := NameValidation(name)
	annotation := map[string]string{
		"customSecretVersion": version,
	}

	//coverting stringdata into data
	var data = make(map[string][]byte)
	for k, v := range secret.StringData {
		_, er := b64.StdEncoding.DecodeString(v)
		if er == nil {
			data[k] = []byte(v)
		} else {
			str := b64.StdEncoding.EncodeToString([]byte(v))
			data[k] = []byte(str)
		}
	}
	for k, v := range secret.Data {
		data[k] = v
	}
	secretAnnotation := make(map[string]string)
	sdata, _ := json.Marshal(secret.Annotations)
	_ = json.Unmarshal(sdata, &secretAnnotation)

	//remove customsecret annotation from version
	delete(secretAnnotation, "currentCustomSecretVersion")
	delete(secretAnnotation, "customSecret-name")
	delete(secretAnnotation, "updateMethod")
	delete(secretAnnotation, "deployments")
	delete(secretAnnotation, "statefulsets")
	cs := &customSecretv1alpha1.CustomSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secret.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(secret, corev1.SchemeGroupVersion.WithKind("Secret")),
			},
			Annotations: annotation,
			Labels:      labels,
		},
		Spec: customSecretv1alpha1.CustomSecretSpec{
			Data:              data,
			Type:              secret.Type,
			SecretName:        secret.Name,
			SecretAnnotations: secretAnnotation,
		},
	}
	return cs, version
}

//Update Secret
func (r *SecretReconciler) UpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	//get CCM latest version
	var secretlogname string = secret.Namespace + "/" + secret.Name
	var csList customSecretv1alpha1.CustomSecretList
	err := r.List(ctx, &csList, client.MatchingLabels{"name": secret.Name, "current": "true"}, client.InNamespace(secret.Namespace))
	if err != nil {
		log.Error(err, secretlogname+" Unable to get customSecret list")
		return err
	}
	if len(csList.Items) == 1 {
		//checking configmap annotation version with currentCCM version
		if csList.Items[0].Annotations["customSecretVersion"] != secret.Annotations["currentCustomSecretVersion"] {
			//calling copy CCM to CM function
			er := r.CopyCSToSecret(ctx, secret)
			if er != nil {
				return er
			}

			//update ccm to remove current
			delete(csList.Items[0].Labels, "current")
			errs := r.Update(ctx, &csList.Items[0])
			if errs != nil {
				r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret: %v", errs)
				return errs
			}
		} else {
			secretAnnotation := make(map[string]string)
			data, _ := json.Marshal(secret.ObjectMeta.Annotations)
			_ = json.Unmarshal(data, &secretAnnotation)
			delete(secretAnnotation, "currentCustomSecretVersion")
			delete(secretAnnotation, "customSecret-name")
			delete(secretAnnotation, "updateMethod")
			delete(secretAnnotation, "deployments")
			delete(secretAnnotation, "statefulsets")
			//content of configMap and customSecret are not same create newCS and make that as current
			if len(csList.Items[0].Spec.SecretAnnotations) != 0 {
				if reflect.DeepEqual(secret.Data, csList.Items[0].Spec.Data) == false || secret.Type != csList.Items[0].Spec.Type || reflect.DeepEqual(secretAnnotation, csList.Items[0].Spec.SecretAnnotations) == false {
					errs := r.CreateNewCS(ctx, secret, &csList.Items[0])
					if errs != nil {
						return errs
					}

				}
			} else {
				if reflect.DeepEqual(secret.Data, csList.Items[0].Spec.Data) == false || secret.Type != csList.Items[0].Spec.Type {
					errs := r.CreateNewCS(ctx, secret, &csList.Items[0])
					if errs != nil {
						return errs
					}
				}
			}
		}
	}
	return nil
}

//copyCStoSecret
func (r *SecretReconciler) CopyCSToSecret(ctx context.Context, secret *corev1.Secret) error {
	var cs customSecretv1alpha1.CustomSecret
	csNameNamespace := types.NamespacedName{
		Namespace: secret.Namespace,
		Name:      secret.Name + "-" + secret.Annotations["currentCustomSecretVersion"],
	}
	er := r.Get(ctx, csNameNamespace, &cs)
	if er != nil {
		r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedGetCustomSecret", "Error Getting CustomSecret: %v", er.Error())
		return er
	}

	//copying content
	secret.Data = cs.Spec.Data
	cs.Spec.SecretAnnotations["deployments"] = secret.Annotations["deployments"]
	cs.Spec.SecretAnnotations["statefulsets"] = secret.Annotations["statefulsets"]
	cs.Spec.SecretAnnotations["updateMethod"] = secret.Annotations["updateMethod"]
	cs.Spec.SecretAnnotations["currentCustomSecretVersion"] = secret.Annotations["currentCustomSecretVersion"]
	cs.Spec.SecretAnnotations["customSecret-name"] = cs.Name
	secret.Annotations = cs.Spec.SecretAnnotations
	//Update configMap content based on configmap version
	err := r.Update(ctx, secret)
	if err != nil {
		return err
	}

	//update CCM with current
	cs.Labels["current"] = "true"
	delete(cs.Spec.SecretAnnotations, "currentCustomSecretVersion")
	delete(cs.Spec.SecretAnnotations, "customSecret-name")
	delete(cs.Spec.SecretAnnotations, "updateMethod")
	delete(cs.Spec.SecretAnnotations, "deployments")
	delete(cs.Spec.SecretAnnotations, "statefulsets")
	errs := r.Update(ctx, &cs)
	if errs != nil {
		r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret: %v", errs)
		return errs
	}

	r.EventRecorder.Eventf(secret, corev1.EventTypeNormal, "secret", "update cs content %v to secret %v", cs.Name, secret.Name)
	return nil
}

func (r *SecretReconciler) CreateNewCS(ctx context.Context, secret *corev1.Secret, currentcs *customSecretv1alpha1.CustomSecret) error {
	//delete current label from previous currentCCM
	delete(currentcs.Labels, "current")
	errs := r.Update(ctx, currentcs)
	if errs != nil {
		r.EventRecorder.Eventf(currentcs, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret in removing curret label level: %v", errs)
		return errs
	}

	//delete latest label from latest ccm
	//getting latest ccm
	latestcs := customSecretv1alpha1.CustomSecretList{}
	for i := 0; i <= 5; i++ {
		err := r.List(ctx, &latestcs, client.MatchingLabels{"name": secret.Name, "latest": "true"}, client.InNamespace(secret.Namespace))
		if err != nil {
			r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret in getting latest label level:: %v", errs)
		}
		delete(latestcs.Items[0].Labels, "latest")
		errs = r.Update(ctx, &latestcs.Items[0])
		if errs != nil {
			r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret in removing latest label level:: %v", errs)
			//reverting the previous change
			if !strings.Contains(errs.Error(), "changes to the latest version and try again") {
				currentcs.Labels["current"] = "true"
				errs := r.Update(ctx, currentcs)
				if errs != nil {
					r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret in adding current label level:: %v", errs)
					return errs
				}
				return errs
			}
		}
		if errs == nil {
			break
		}
	}

	csNew, version := newCustomSecret(secret)
	er := r.Create(ctx, csNew)
	if er != nil {
		r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedCreateCustomSecret", "Error creating CustomSecret: %v", er)
		//reverting the previous change
		currentcs.Labels["current"] = "true"
		errs := r.Update(ctx, currentcs)
		if errs != nil {
			r.EventRecorder.Eventf(currentcs, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret: %v", errs)
			return errs
		}
		latestcs.Items[0].Labels["latest"] = "true"
		errs = r.Update(ctx, &latestcs.Items[0])
		if errs != nil {
			r.EventRecorder.Eventf(secret, corev1.EventTypeWarning, "FailedUpdatingCustomSecret", "Error Updating CustomSecret: %v", errs)
			return errs
		}
		return er
	}

	//update config map with version and ccm name
	secret.Annotations["currentCustomSecretVersion"] = version
	secret.Annotations["customSecret-name"] = csNew.Name
	secret.Annotations["updateMethod"] = "ignoreWhenShared"
	err := r.Update(ctx, secret)
	if err != nil {
		return err
	}
	r.EventRecorder.Eventf(secret, corev1.EventTypeNormal, "updateSecret", "update cs version %v and name %v", version, csNew.Name)

	//trigger rolling Update for kind=deployment
	if secret.Annotations["deployments"] != "" {
		annotation := secret.Annotations["deployments"]
		split := strings.Split(annotation, ",")
		if secret.Annotations["updateMethod"] == "ignoreWhenShared" {
			if len(split) > 1 {
				klog.Error("can't trigger rolling update updateMethod is ignoreWhenShared")
				return errors.NewBadRequest("can't trigger rolling update updateMethod is ignoreWhenShared")
			} else {
				var deployment appsV1.Deployment
				deployNameNamespace := types.NamespacedName{
					Namespace: secret.Namespace,
					Name:      split[0],
				}
				err := r.Get(ctx, deployNameNamespace, &deployment)
				if err != nil {
					klog.Error("Failed on getting deployment '%s' Error: %s", split[0], err.Error)
					return err
				}
				//update new version
				deployment.Spec.Template.Annotations["cs-"+secret.Name] = version
				err = r.Update(ctx, &deployment)
				if err != nil {
					klog.Error("Failed on updating deployment '%s' Error: %s", split[0], err.Error)
					return err
				}
			}
		}
	}
	if secret.Annotations["statefulsets"] != "" {
		annotation := secret.Annotations["statefulsets"]
		split := strings.Split(annotation, ",")
		if secret.Annotations["updateMethod"] == "ignoreWhenShared" {
			if len(split) > 1 {
				klog.Error("can't trigger rolling update updateMethod is ignoreWhenShared")
				return errors.NewBadRequest("can't trigger rolling update updateMethod is ignoreWhenShared")
			} else {
				var sts appsV1.StatefulSet
				stsNameNamespace := types.NamespacedName{
					Namespace: secret.Namespace,
					Name:      split[0],
				}

				err := r.Get(ctx, stsNameNamespace, &sts)
				if err != nil {
					klog.Error("Failed on getting statefulset '%s' Error: %s", split[0], err.Error)
					return err
				}
				//update new version
				sts.Spec.Template.Annotations["cs-"+secret.Name] = version
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
