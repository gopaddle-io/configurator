package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	clientset "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

//validation webhook
func (whsvr *WebhookServer) PodConfigController(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		klog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	var admissionResponse *v1.AdmissionResponse
	ar := v1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		klog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = ConfigValidation(&ar)
	}

	if admissionResponse != nil {
		ar.Response = admissionResponse
		if ar.Request != nil {
			ar.Response.UID = ar.Request.UID
		}
	}
	resp, err := json.Marshal(ar)
	if err != nil {
		klog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	klog.Infof("Ready to write reponse ...", string(resp))
	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func ConfigValidation(ar *v1.AdmissionReview) *v1.AdmissionResponse {

	req := ar.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Info("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v validateOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			//reading configmapVersion from configmap
			//get clusterConf
			var cfg *rest.Config
			var err error
			cfg, err = rest.InClusterConfig()
			//create clientset
			clientSet, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				klog.Error("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
			}

			configMap, err := clientSet.CoreV1().ConfigMaps(pod.Namespace).Get(context.TODO(), volume.ConfigMap.Name, metav1.GetOptions{})
			if err != nil {
				return &v1.AdmissionResponse{
					Result: &metav1.Status{
						Message: err.Error(),
					},
				}
			}
			if pod.Annotations["ccm-"+volume.ConfigMap.Name] == configMap.Annotations["currentCustomConfigMapVersion"] {
				klog.Info("customConfigMap version is equal to pod configVersion")
			} else {
				//copy configMap
				configMap.Annotations["currentCustomConfigMapVersion"] = pod.Annotations["ccm-"+volume.ConfigMap.Name]
				err := CopyCCMToCM(configMap)
				if err != nil {
					return &v1.AdmissionResponse{
						Result: &metav1.Status{
							Message: err.Error(),
						},
					}
				}
			}

		} else if volume.Secret != nil {
			//get clusterConf
			var cfg *rest.Config
			var err error
			cfg, err = rest.InClusterConfig()
			//create clientset
			clientSet, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				klog.Error("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
			}

			secret, err := clientSet.CoreV1().Secrets(pod.Namespace).Get(context.TODO(), volume.Secret.SecretName, metav1.GetOptions{})
			if err != nil {
				return &v1.AdmissionResponse{
					Result: &metav1.Status{
						Message: err.Error(),
					},
				}
			}
			if pod.Annotations["cs-"+volume.Secret.SecretName] == secret.Annotations["currentCustomSecretVersion"] {
				klog.Info("customSecret version is equal to pod SecretVersion")
			} else {
				//copy CS to secret
				secret.Annotations["currentCustomSecretVersion"] = pod.Annotations["cs-"+volume.Secret.SecretName]
				err := CopyCSToSecret(secret)
				if err != nil {
					return &v1.AdmissionResponse{
						Result: &metav1.Status{
							Message: err.Error(),
						},
					}
				}
			}
		}
	}
	return &v1.AdmissionResponse{
		Allowed: true,
	}
}

func CopyCCMToCM(configmap *corev1.ConfigMap) error {
	var cfg *rest.Config
	var err error
	cfg, err = rest.InClusterConfig()
	//create clientset
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Error("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
		return err
	}
	configuratorClientSet, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Error("Error building example clientset: %s", err.Error())
		return err
	}

	ccm, err := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(configmap.Namespace).Get(context.TODO(), configmap.Name+"-"+configmap.Annotations["currentCustomConfigMapVersion"], metav1.GetOptions{})
	if err != nil {
		klog.Error("Error getting ccm: %s", err.Error())
		return err
	}
	//copying content
	configmap.Data = ccm.Spec.Data
	configmap.BinaryData = ccm.Spec.BinaryData
	configmap.Annotations["customConfigMap-name"] = ccm.Name
	//Update configMap content based on configmap version
	_, err = clientSet.CoreV1().ConfigMaps(configmap.Namespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	//getting current ccm
	label := "name=config-multi-env-files,current=true"
	listOption := metav1.ListOptions{LabelSelector: label}
	currentccm, _ := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(configmap.Namespace).List(context.TODO(), listOption)
	currentCCM := currentccm.Items[0]
	//remove current label from currentCCM
	delete(currentCCM.Labels, "current")
	_, errs := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(configmap.Namespace).Update(context.TODO(), &currentCCM, metav1.UpdateOptions{})
	if errs != nil {
		return errs
	}

	//add currentCCM
	ccm.Labels["current"] = "true"
	_, errs = configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(configmap.Namespace).Update(context.TODO(), ccm, metav1.UpdateOptions{})
	if errs != nil {
		return errs
	}
	return nil
}

//copy customSecret to Secret
func CopyCSToSecret(secret *corev1.Secret) error {
	var cfg *rest.Config
	var err error
	cfg, err = rest.InClusterConfig()
	//create clientset
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Error("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
		return err
	}
	configuratorClientSet, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Error("Error building example clientset: %s", err.Error())
		return err
	}

	cs, err := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(secret.Namespace).Get(context.TODO(), secret.Name+"-"+secret.Annotations["currentCustomSecretVersion"], metav1.GetOptions{})
	if err != nil {
		klog.Error("Error getting ccm: %s", err.Error())
		return err
	}
	//copying content
	secret.Data = cs.Spec.Data
	cs.Spec.SecretAnnotations["customConfigMap-name"] = cs.Name
	cs.Spec.SecretAnnotations["deployments"] = secret.Annotations["deployments"]
	cs.Spec.SecretAnnotations["statefulsets"] = secret.Annotations["statefulsets"]
	cs.Spec.SecretAnnotations["updateMethod"] = secret.Annotations["updateMethod"]
	cs.Spec.SecretAnnotations["currentCustomSecretVersion"] = secret.Annotations["currentCustomSecretVersion"]
	secret.Annotations = cs.Spec.SecretAnnotations
	//Update secret content based on secret version
	_, err = clientSet.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	//getting current ccm
	label := "name=config-multi-env-files,current=true"
	listOption := metav1.ListOptions{LabelSelector: label}
	currentcs, _ := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(secret.Namespace).List(context.TODO(), listOption)
	currentCS := currentcs.Items[0]
	//remove current label from currentCCM
	delete(currentCS.Labels, "current")
	_, errs := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(secret.Namespace).Update(context.TODO(), &currentCS, metav1.UpdateOptions{})
	if errs != nil {
		return errs
	}

	//add currentCCM
	cs.Labels["current"] = "true"
	delete(cs.Spec.SecretAnnotations, "currentCustomSecretVersion")
	delete(cs.Spec.SecretAnnotations, "customSecret-name")
	delete(cs.Spec.SecretAnnotations, "updateMethod")
	delete(cs.Spec.SecretAnnotations, "deployments")
	delete(cs.Spec.SecretAnnotations, "statefulsets")
	_, errs = configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(secret.Namespace).Update(context.TODO(), cs, metav1.UpdateOptions{})
	if errs != nil {
		return errs
	}
	return nil
}
