package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	customConfigMapv1alpha1 "github.com/gopaddle-io/configurator/pkg/apis/configuratorcontroller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// configMapsyncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the customSecret resource
// with the current status of the resource.
func (c *Controller) configMapSyncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the configmap resource with this namespace/name
	configMap, err := c.configmapsLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		// The ConfigMap resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("configMap '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	ccm_Version := configMap.Annotations["currentCustomConfigMapVersion"]
	// check the CCM_version in the configMap if the Version not exist.
	// it create new version of CCM and add annotation to that configMap
	if ccm_Version == "" {
		ccm, version := newCustomConfigMap(configMap)
		ccm_data, er := c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(ccm.Namespace).Create(context.TODO(), ccm, metav1.CreateOptions{})
		if er != nil {
			c.recorder.Eventf(ccm, corev1.EventTypeWarning, "FailedCreateCustomConfigMap", "Error creating CustomConfigMap: %v", err)
			return er
		}

		//update config map with version and ccm name
		configMap.Annotations["currentCustomConfigMapVersion"] = version
		configMap.Annotations["customConfigMap-name"] = ccm_data.Name
		configMap.Annotations["updateMethod"] = "ignoreWhenShared"
		_, err := c.kubeclientset.CoreV1().ConfigMaps(configMap.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		c.recorder.Eventf(configMap, corev1.EventTypeNormal, "configMapVersionUpdate", "Add ccm version and name configMap: %v", configMap.Name)
		return nil
	} else {
		fmt.Println("comes to update config part *************************")
		er := c.UpdateConfigMap(configMap)
		if er != nil {
			c.recorder.Eventf(configMap, corev1.EventTypeNormal, "FailedCreateCustomConfigMapVersion", "Error in creating CustomConfigMap: %v", er.Error())
			return err
		}
	}

	//c.recorder.Event(customSecret, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
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

//copyCCMtoCM
func (c *Controller) CopyCCMToCM(configmap *corev1.ConfigMap) error {
	ccm, _ := c.customConfigMapLister.CustomConfigMaps(configmap.Namespace).Get(configmap.Name + "-" + configmap.Annotations["currentCustomConfigMapVersion"])
	//copying content
	configmap.Data = ccm.Spec.Data
	configmap.BinaryData = ccm.Spec.BinaryData
	//Update configMap content based on configmap version
	_, err := c.kubeclientset.CoreV1().ConfigMaps(configmap.Namespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	//update CCM with current
	ccm.Labels["current"] = "true"
	ccm_update, errs := c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(ccm.Namespace).Update(context.TODO(), ccm, metav1.UpdateOptions{})
	if errs != nil {
		c.recorder.Eventf(ccm_update, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
		return errs
	}

	c.recorder.Eventf(configmap, corev1.EventTypeNormal, "configMap", "update ccm content %v to configMap %v", ccm.Name, configmap.Name)
	return nil
}

//Update ConfigMap
func (c *Controller) UpdateConfigMap(configMap *corev1.ConfigMap) error {
	//get CCM latest version
	labels := make(map[string]string)
	labels["name"] = configMap.Name
	labels["current"] = "true"
	selector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: labels,
	})
	ccm, _ := c.customConfigMapLister.CustomConfigMaps(configMap.Namespace).List(selector)
	if len(ccm) == 1 {
		//checking configmap annotation version with latest CCM version
		//if both are not same compare the content.
		if ccm[0].Annotations["customConfigMapVersion"] != configMap.Annotations["currentCustomConfigMapVersion"] {
			//check the content are equal
			//ccmName := configMap.Name + "-" + configMap.Annotations["customConfigMapVersion"]
			//customCM, _ := c.customConfigMapLister.CustomConfigMaps(configMap.Namespace).Get(ccmName)
			//if reflect.DeepEqual(configMap.Data, customCM.Spec.Data) == false || reflect.DeepEqual(configMap.BinaryData, customCM.Spec.BinaryData) == false {
			//calling copy CCM to CM function
			er := c.CopyCCMToCM(configMap)
			if er != nil {
				return er
			}

			//update ccm to remove current
			delete(ccm[0].Labels, "current")
			ccm_update, errs := c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(ccm[0].Namespace).Update(context.TODO(), ccm[0], metav1.UpdateOptions{})
			if errs != nil {
				c.recorder.Eventf(ccm_update, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
				return errs
			}
			//create new revision update CM trigger rolling update
			// errs := c.CreateNewCCM(configMap, ccm[0])
			// if errs != nil {
			// 	return errs
			// }
			//}
		} else {
			if reflect.DeepEqual(configMap.Data, ccm[0].Spec.Data) == false || reflect.DeepEqual(configMap.BinaryData, ccm[0].Spec.BinaryData) == false {
				errs := c.CreateNewCCM(configMap, ccm[0])
				if errs != nil {
					return errs
				}

			}
		}
	}
	return nil
}

func (c *Controller) CreateNewCCM(configMap *corev1.ConfigMap, currentccm *customConfigMapv1alpha1.CustomConfigMap) error {
	//delete current label from previous currentCCM
	delete(currentccm.Labels, "current")
	ccm_update, errs := c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(currentccm.Namespace).Update(context.TODO(), currentccm, metav1.UpdateOptions{})
	if errs != nil {
		c.recorder.Eventf(ccm_update, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
		return errs
	}

	//delete latest label from latest ccm
	//getting latest ccm
	labels := make(map[string]string)
	labels["name"] = configMap.Name
	labels["latest"] = "true"
	selector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: labels,
	})
	latestccm, _ := c.customConfigMapLister.CustomConfigMaps(configMap.Namespace).List(selector)
	delete(latestccm[0].Labels, "latest")
	ccm_update, errs = c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(latestccm[0].Namespace).Update(context.TODO(), latestccm[0], metav1.UpdateOptions{})
	if errs != nil {
		c.recorder.Eventf(ccm_update, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
		//reverting the previous change
		currentccm.Annotations["current"] = "true"
		ccm_update, errs := c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(currentccm.Namespace).Update(context.TODO(), currentccm, metav1.UpdateOptions{})
		if errs != nil {
			c.recorder.Eventf(ccm_update, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
			return errs
		}
		return errs
	}

	ccmNew, version := newCustomConfigMap(configMap)
	ccm_data, er := c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(ccmNew.Namespace).Create(context.TODO(), ccmNew, metav1.CreateOptions{})
	if er != nil {
		c.recorder.Eventf(ccmNew, corev1.EventTypeWarning, "FailedCreateCustomConfigMap", "Error creating CustomConfigMap: %v", er)
		//reverting the previous change
		currentccm.Annotations["current"] = "true"
		ccm_update, errs := c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(currentccm.Namespace).Update(context.TODO(), currentccm, metav1.UpdateOptions{})
		if errs != nil {
			c.recorder.Eventf(ccm_update, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
			return errs
		}
		latestccm[0].Annotations["latest"] = "true"
		ccm_update, errs = c.sampleclientset.ConfiguratorV1alpha1().CustomConfigMaps(latestccm[0].Namespace).Update(context.TODO(), latestccm[0], metav1.UpdateOptions{})
		if errs != nil {
			c.recorder.Eventf(ccm_update, corev1.EventTypeWarning, "FailedUpdatingCustomConfigMap", "Error Updating CustomConfigMap: %v", errs)
			return errs
		}
		return er
	}

	//update config map with version and ccm name
	configMap.Annotations["currentCustomConfigMapVersion"] = version
	configMap.Annotations["customConfigMap-name"] = ccm_data.Name
	configMap.Annotations["updateMethod"] = "ignoreWhenShared"
	_, err := c.kubeclientset.CoreV1().ConfigMaps(configMap.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	c.recorder.Eventf(configMap, corev1.EventTypeNormal, "updateConfigMap", "update ccm version %v and name %v", version, ccm_data.Name)
	fmt.Println("before triggering ccm")
	//TODO:- trigger rollingUpdate

	//trigger rolling Update for kind=deployment
	if configMap.Annotations["deployments"] != "" {
		annotation := configMap.Annotations["deployments"]
		split := strings.Split(annotation, ",")
		if configMap.Annotations["updateMethod"] == "ignoreWhenShared" {
			if len(split) > 1 {
				klog.Error("can't trigger rolling update updateMethod is ignoreWhenShared")
				return errors.NewBadRequest("can't trigger rolling update updateMethod is ignoreWhenShared")
			} else {
				deployment, err := c.kubeclientset.AppsV1().Deployments(configMap.Namespace).Get(context.TODO(), split[0], metav1.GetOptions{})
				if err != nil {
					return err
				}
				//update new version
				deployment.Spec.Template.Annotations["ccm-"+configMap.Name] = version

				deployment, err = c.kubeclientset.AppsV1().Deployments(configMap.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
