package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	v1 "k8s.io/api/admission/v1"
	appsV1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func (whsvr *WebhookServer) StatefulSetController(w http.ResponseWriter, r *http.Request) {

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
		admissionResponse = statefulsetMutate(&ar)
	}
	admissionReview := v1.AdmissionReview{}
	admissionReview.Kind = "AdmissionReview"
	admissionReview.APIVersion = "admission.k8s.io/v1"
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}
	resp, err := json.Marshal(admissionReview)
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

//statefulsetmutate it create the AdmisionResponse of statefulset patch
func statefulsetMutate(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	req := ar.Request
	var statefulset appsV1.StatefulSet
	if err := json.Unmarshal(req.Object.Raw, &statefulset); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Info("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, statefulset.Name, req.UID, req.Operation, req.UserInfo)

	patchBytes, err := createStatefulsetPatch(&statefulset)
	if err != nil {
		glog.Infof("AdmissionResponse: create patch failed %v\n", err.Error())
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Info("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &v1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1.PatchType {
			pt := v1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

//createStatefulsetPatch it create a statefulset patch
func createStatefulsetPatch(statefulset *appsV1.StatefulSet) ([]byte, error) {
	var patch []patchOperation
	addnewAnnotation := make(map[string]string)
	for _, volume := range statefulset.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil {
			//check already configMapname exist or not
			if statefulset.Spec.Template.Annotations["ccm-"+volume.ConfigMap.Name] == "" || len(statefulset.Spec.Template.Annotations) == 0 {
				//reading configmapVersion from configmap
				//get clusterConf
				var cfg *rest.Config
				var err error
				cfg, err = rest.InClusterConfig()
				//create clientset
				clientSet, err := kubernetes.NewForConfig(cfg)
				if err != nil {
					klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
				}
				configMap, err := clientSet.CoreV1().ConfigMaps(statefulset.Namespace).Get(context.TODO(), volume.ConfigMap.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				//adding annotation to configMap
				if configMap.Annotations["statefulsets"] == "" {
					if len(configMap.Annotations) == 0 {
						annotation := make(map[string]string)
						annotation["statefulsets"] = statefulset.Name
						configMap.Annotations = annotation
					} else {
						configMap.Annotations["statefulsets"] = statefulset.Name
					}
				} else {
					// check that deployment name already exist in configMapName
					annotation := configMap.Annotations["statefulsets"]
					split := strings.Split(annotation, ",")
					check := false
					for _, s := range split {
						if s == statefulset.Name {
							check = true
						}
					}
					if !check {
						configMap.Annotations["statefulsets"] = statefulset.Name
					}
				}
				configMap, errs := clientSet.CoreV1().ConfigMaps(statefulset.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
				if errs != nil {
					return nil, errs
				}
				addnewAnnotation["ccm-"+configMap.Name] = configMap.Annotations["currentCustomConfigMapVersion"]
			}
		} else if volume.Secret != nil {
			if statefulset.Spec.Template.Annotations["cs-"+volume.Secret.SecretName] == "" || len(statefulset.Spec.Template.Annotations) == 0 {
				//reading SecretVersion from Secret
				//get clusterConf
				var cfg *rest.Config
				var err error
				cfg, err = rest.InClusterConfig()
				//create clientset
				clientSet, err := kubernetes.NewForConfig(cfg)
				if err != nil {
					klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
				}
				secret, err := clientSet.CoreV1().Secrets(statefulset.Namespace).Get(context.TODO(), volume.Secret.SecretName, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}

				//adding annotation to configMap
				if secret.Annotations["statefulsets"] == "" {
					if len(secret.Annotations) == 0 {
						annotation := make(map[string]string)
						annotation["statefulsets"] = statefulset.Name
						secret.Annotations = annotation
					} else {
						secret.Annotations["statefulsets"] = statefulset.Name
					}
				} else {
					// check that deployment name already exist in secretName
					annotation := secret.Annotations["statefulsets"]
					split := strings.Split(annotation, ",")
					check := false
					for _, s := range split {
						if s == statefulset.Name {
							check = true
						}
					}
					if !check {
						secret.Annotations["statefulsets"] = statefulset.Name
					}
				}
				secret, errs := clientSet.CoreV1().Secrets(statefulset.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
				if errs != nil {
					return nil, errs
				}
				addnewAnnotation["cs-"+secret.Name] = secret.Annotations["currentCustomSecretVersion"]
			}
		}
	}

	//add new annotation form envfrom container
	for _, container := range statefulset.Spec.Template.Spec.Containers {
		for _, env := range container.EnvFrom {
			if env.ConfigMapRef != nil {
				//check already configMapname exist or not
				if statefulset.Spec.Template.Annotations["ccm-"+env.ConfigMapRef.Name] == "" || len(statefulset.Spec.Template.Annotations) == 0 {
					//reading configmapVersion from configmap
					//get clusterConf
					var cfg *rest.Config
					var err error
					cfg, err = rest.InClusterConfig()
					//create clientset
					clientSet, err := kubernetes.NewForConfig(cfg)
					if err != nil {
						klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
					}
					configMap, err := clientSet.CoreV1().ConfigMaps(statefulset.Namespace).Get(context.TODO(), env.ConfigMapRef.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					//adding annotation to configMap
					if configMap.Annotations["statefulsets"] == "" {
						if len(configMap.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["statefulsets"] = statefulset.Name
							configMap.Annotations = annotation
						} else {
							configMap.Annotations["statefulsets"] = statefulset.Name
						}
					} else {
						// check that deployment name already exist in configMapName
						annotation := configMap.Annotations["statefulsets"]
						split := strings.Split(annotation, ",")
						check := false
						for _, s := range split {
							if s == statefulset.Name {
								check = true
							}
						}
						if !check {
							configMap.Annotations["statefulsets"] = statefulset.Name
						}
					}
					configMap, errs := clientSet.CoreV1().ConfigMaps(statefulset.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
					if errs != nil {
						return nil, errs
					}
					addnewAnnotation["ccm-"+configMap.Name] = configMap.Annotations["currentCustomConfigMapVersion"]
				}
			} else if env.SecretRef != nil {
				if statefulset.Spec.Template.Annotations["cs-"+env.SecretRef.Name] == "" || len(statefulset.Spec.Template.Annotations) == 0 {
					//reading SecretVersion from Secret
					//get clusterConf
					var cfg *rest.Config
					var err error
					cfg, err = rest.InClusterConfig()
					//create clientset
					clientSet, err := kubernetes.NewForConfig(cfg)
					if err != nil {
						klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
					}
					secret, err := clientSet.CoreV1().Secrets(statefulset.Namespace).Get(context.TODO(), env.SecretRef.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}

					//adding annotation to configMap
					if secret.Annotations["statefulsets"] == "" {
						if len(secret.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["statefulsets"] = statefulset.Name
							secret.Annotations = annotation
						} else {
							secret.Annotations["statefulsets"] = statefulset.Name
						}
					} else {
						// check that deployment name already exist in secretName
						annotation := secret.Annotations["statefulsets"]
						split := strings.Split(annotation, ",")
						check := false
						for _, s := range split {
							if s == statefulset.Name {
								check = true
							}
						}
						if !check {
							secret.Annotations["statefulsets"] = statefulset.Name
						}
					}
					secret, errs := clientSet.CoreV1().Secrets(statefulset.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
					if errs != nil {
						return nil, errs
					}
					addnewAnnotation["cs-"+secret.Name] = secret.Annotations["currentCustomSecretVersion"]
				}
			}
		}
	}
	//remove annotation in deployment if that configmap name is not there
	statefulsetAnnotation := make(map[string]string)
	removeAnnotation := make(map[string]string)
	if len(statefulset.Spec.Template.Annotations) != 0 {
		for key, value := range statefulset.Spec.Template.Annotations {
			if strings.Contains(key, "ccm-") {
				check := false
				for _, volume := range statefulset.Spec.Template.Spec.Volumes {
					if volume.ConfigMap != nil {
						if key == "ccm-"+volume.ConfigMap.Name {
							check = true
							statefulsetAnnotation[key] = value
						}
					}
				}
				for _, container := range statefulset.Spec.Template.Spec.Containers {
					for _, env := range container.EnvFrom {
						if env.ConfigMapRef != nil {
							if key == "ccm-"+env.ConfigMapRef.Name {
								check = true
								statefulsetAnnotation[key] = value
							}
						}
					}
				}

				for _, initContainer := range statefulset.Spec.Template.Spec.InitContainers {
					for _, env := range initContainer.EnvFrom {
						if env.ConfigMapRef != nil {
							if key == "ccm-"+env.ConfigMapRef.Name {
								check = true
								statefulsetAnnotation[key] = value
							}
						}
					}
				}

				if !check {
					removeAnnotation[key] = value
				}
			} else if strings.Contains(key, "cs-") {
				check := false
				for _, volume := range statefulset.Spec.Template.Spec.Volumes {
					if volume.Secret != nil {
						if key == "cs-"+volume.Secret.SecretName {
							check = true
							statefulsetAnnotation[key] = value
						}
					}
				}
				for _, container := range statefulset.Spec.Template.Spec.Containers {
					for _, env := range container.EnvFrom {
						if env.SecretRef != nil {
							if key == "cs-"+env.SecretRef.Name {
								check = true
								statefulsetAnnotation[key] = value
							}
						}
					}
				}
				for _, initContainer := range statefulset.Spec.Template.Spec.InitContainers {
					for _, env := range initContainer.EnvFrom {
						if env.SecretRef != nil {
							if key == "cs-"+env.SecretRef.Name {
								check = true
								statefulsetAnnotation[key] = value
							}
						}
					}
				}
				if !check {
					removeAnnotation[key] = value
				}
			} else {
				statefulsetAnnotation[key] = value
			}
		}
	}
	patch = append(patch, updateAnnotation(statefulsetAnnotation, addnewAnnotation, removeAnnotation)...)
	return json.Marshal(patch)
}
