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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

func (whsvr *WebhookServer) DeployController(w http.ResponseWriter, r *http.Request) {
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
		admissionResponse = deployMutate(&ar)
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

// main mutation process
func deployMutate(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	req := ar.Request
	var deployment appsV1.Deployment
	if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Info("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, deployment.Name, req.UID, req.Operation, req.UserInfo)

	patchBytes, err := createDeploymentPatch(&deployment)
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

func createDeploymentPatch(deployment *appsV1.Deployment) ([]byte, error) {
	var patch []patchOperation
	addnewAnnotation := make(map[string]string)
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil {
			//check already configMapname exist or not
			if deployment.Spec.Template.Annotations["ccm-"+volume.ConfigMap.Name] == "" || len(deployment.Spec.Template.Annotations) == 0 {
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
				configMap, err := clientSet.CoreV1().ConfigMaps(deployment.Namespace).Get(context.TODO(), volume.ConfigMap.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				//adding annotation to configMap
				if configMap.Annotations["deployments"] == "" {
					if len(configMap.Annotations) == 0 {
						annotation := make(map[string]string)
						annotation["deployments"] = deployment.Name
						configMap.Annotations = annotation
					} else {
						configMap.Annotations["deployments"] = deployment.Name
					}
				} else {
					// check that deployment name already exist in configMapName
					annotation := configMap.Annotations["deployments"]
					split := strings.Split(annotation, ",")
					check := false
					for _, s := range split {
						if s == deployment.Name {
							check = true
						}
					}
					if !check {
						configMap.Annotations["deployments"] = configMap.Annotations["deployments"] + "," + deployment.Name
					}
				}
				configMap, errs := clientSet.CoreV1().ConfigMaps(deployment.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
				if errs != nil {
					return nil, errs
				}
				addnewAnnotation["ccm-"+configMap.Name] = configMap.Annotations["currentCustomConfigMapVersion"]
			}
		} else if volume.Secret != nil {
			//check already secretName exist or not
			if deployment.Spec.Template.Annotations["cs-"+volume.Secret.SecretName] == "" || len(deployment.Spec.Template.Annotations) == 0 {
				//reading secretVersion from Secret
				//get clusterConf
				var cfg *rest.Config
				var err error
				cfg, err = rest.InClusterConfig()
				//create clientset
				clientSet, err := kubernetes.NewForConfig(cfg)
				if err != nil {
					klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
				}
				secret, err := clientSet.CoreV1().Secrets(deployment.Namespace).Get(context.TODO(), volume.Secret.SecretName, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				//adding annotation to secret
				if secret.Annotations["deployments"] == "" {
					if len(secret.Annotations) == 0 {
						annotation := make(map[string]string)
						annotation["deployments"] = deployment.Name
						secret.Annotations = annotation
					} else {
						secret.Annotations["deployments"] = deployment.Name
					}
				} else {
					// check that deployment name already exist in configMapName
					annotation := secret.Annotations["deployments"]
					split := strings.Split(annotation, ",")
					check := false
					for _, s := range split {
						if s == deployment.Name {
							check = true
						}
					}
					if !check {
						secret.Annotations["deployments"] = secret.Annotations["deployments"] + "," + deployment.Name
					}
				}
				secret, errs := clientSet.CoreV1().Secrets(deployment.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
				if errs != nil {
					return nil, errs
				}
				addnewAnnotation["cs-"+secret.Name] = secret.Annotations["currentCustomSecretVersion"]

			}
		}
	}
	//add new annotation form envfrom container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, env := range container.EnvFrom {
			if env.ConfigMapRef != nil {
				if deployment.Spec.Template.Annotations["ccm-"+env.ConfigMapRef.Name] == "" || len(deployment.Spec.Template.Annotations) == 0 {
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
					configMap, err := clientSet.CoreV1().ConfigMaps(deployment.Namespace).Get(context.TODO(), env.ConfigMapRef.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					//adding annotation to configMap
					if configMap.Annotations["deployments"] == "" {
						if len(configMap.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["deployments"] = deployment.Name
							configMap.Annotations = annotation
						} else {
							configMap.Annotations["deployments"] = deployment.Name
						}
					} else {
						// check that deployment name already exist in configMapName
						annotation := configMap.Annotations["deployments"]
						split := strings.Split(annotation, ",")
						check := false
						for _, s := range split {
							if s == deployment.Name {
								check = true
							}
						}
						if !check {
							configMap.Annotations["deployments"] = configMap.Annotations["deployments"] + "," + deployment.Name
						}
					}
					configMap, errs := clientSet.CoreV1().ConfigMaps(deployment.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
					if errs != nil {
						return nil, errs
					}
					addnewAnnotation["ccm-"+configMap.Name] = configMap.Annotations["currentCustomConfigMapVersion"]
				}
			} else if env.SecretRef != nil {
				//check already secretName exist or not
				if deployment.Spec.Template.Annotations["cs-"+env.SecretRef.Name] == "" || len(deployment.Spec.Template.Annotations) == 0 {
					//reading secretVersion from Secret
					//get clusterConf
					var cfg *rest.Config
					var err error
					cfg, err = rest.InClusterConfig()
					//create clientset
					clientSet, err := kubernetes.NewForConfig(cfg)
					if err != nil {
						klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
					}
					secret, err := clientSet.CoreV1().Secrets(deployment.Namespace).Get(context.TODO(), env.SecretRef.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					//adding annotation to secret
					if secret.Annotations["deployments"] == "" {
						if len(secret.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["deployments"] = deployment.Name
							secret.Annotations = annotation
						} else {
							secret.Annotations["deployments"] = deployment.Name
						}
					} else {
						// check that deployment name already exist in configMapName
						annotation := secret.Annotations["deployments"]
						split := strings.Split(annotation, ",")
						check := false
						for _, s := range split {
							if s == deployment.Name {
								check = true
							}
						}
						if !check {
							secret.Annotations["deployments"] = secret.Annotations["deployments"] + "," + deployment.Name
						}
					}
					secret, errs := clientSet.CoreV1().Secrets(deployment.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
					if errs != nil {
						return nil, errs
					}
					addnewAnnotation["cs-"+secret.Name] = secret.Annotations["currentCustomSecretVersion"]

				}
			}
		}
	}

	//add new annotation form envfrom initContainer
	for _, initContainer := range deployment.Spec.Template.Spec.InitContainers {
		for _, env := range initContainer.EnvFrom {
			if env.ConfigMapRef != nil {
				if deployment.Spec.Template.Annotations["ccm-"+env.ConfigMapRef.Name] == "" || len(deployment.Spec.Template.Annotations) == 0 {
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
					configMap, err := clientSet.CoreV1().ConfigMaps(deployment.Namespace).Get(context.TODO(), env.ConfigMapRef.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					//adding annotation to configMap
					if configMap.Annotations["deployments"] == "" {
						if len(configMap.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["deployments"] = deployment.Name
							configMap.Annotations = annotation
						} else {
							configMap.Annotations["deployments"] = deployment.Name
						}
					} else {
						// check that deployment name already exist in configMapName
						annotation := configMap.Annotations["deployments"]
						split := strings.Split(annotation, ",")
						check := false
						for _, s := range split {
							if s == deployment.Name {
								check = true
							}
						}
						if !check {
							configMap.Annotations["deployments"] = configMap.Annotations["deployments"] + "," + deployment.Name
						}
					}
					configMap, errs := clientSet.CoreV1().ConfigMaps(deployment.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
					if errs != nil {
						return nil, errs
					}
					addnewAnnotation["ccm-"+configMap.Name] = configMap.Annotations["currentCustomConfigMapVersion"]
				}
			} else if env.SecretRef != nil {
				//check already secretName exist or not
				if deployment.Spec.Template.Annotations["cs-"+env.SecretRef.Name] == "" || len(deployment.Spec.Template.Annotations) == 0 {
					//reading secretVersion from Secret
					//get clusterConf
					var cfg *rest.Config
					var err error
					cfg, err = rest.InClusterConfig()
					//create clientset
					clientSet, err := kubernetes.NewForConfig(cfg)
					if err != nil {
						klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
					}
					secret, err := clientSet.CoreV1().Secrets(deployment.Namespace).Get(context.TODO(), env.SecretRef.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					//adding annotation to secret
					if secret.Annotations["deployments"] == "" {
						if len(secret.Annotations) == 0 {
							annotation := make(map[string]string)
							annotation["deployments"] = deployment.Name
							secret.Annotations = annotation
						} else {
							secret.Annotations["deployments"] = deployment.Name
						}
					} else {
						// check that deployment name already exist in configMapName
						annotation := secret.Annotations["deployments"]
						split := strings.Split(annotation, ",")
						check := false
						for _, s := range split {
							if s == deployment.Name {
								check = true
							}
						}
						if !check {
							secret.Annotations["deployments"] = secret.Annotations["deployments"] + "," + deployment.Name
						}
					}
					secret, errs := clientSet.CoreV1().Secrets(deployment.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
					if errs != nil {
						return nil, errs
					}
					addnewAnnotation["cs-"+secret.Name] = secret.Annotations["currentCustomSecretVersion"]

				}
			}
		}
	}

	//remove annotation in deployment if that configmap name is not there
	deploymentAnnotation := make(map[string]string)
	removeAnnotation := make(map[string]string)
	if len(deployment.Spec.Template.Annotations) != 0 {
		for key, value := range deployment.Spec.Template.Annotations {
			//checking customConfigMap annotation
			if strings.Contains(key, "ccm-") {
				check := false
				//check configMap Name in volume
				for _, volume := range deployment.Spec.Template.Spec.Volumes {
					if volume.ConfigMap != nil {
						if key == "ccm-"+volume.ConfigMap.Name {
							check = true
							deploymentAnnotation[key] = value
						}
					}
				}
				//check configMap Name in container
				for _, container := range deployment.Spec.Template.Spec.Containers {
					for _, env := range container.EnvFrom {
						if env.ConfigMapRef != nil {
							if key == "ccm-"+env.ConfigMapRef.Name {
								check = true
								deploymentAnnotation[key] = value
							}
						}
					}
				}

				//check configMap Name in initContainer
				for _, initContainer := range deployment.Spec.Template.Spec.InitContainers {
					for _, env := range initContainer.EnvFrom {
						if env.ConfigMapRef != nil {
							if key == "ccm-"+env.ConfigMapRef.Name {
								check = true
								deploymentAnnotation[key] = value
							}
						}
					}
				}
				if !check {
					removeAnnotation[key] = value
				}
			} else if strings.Contains(key, "cs-") { //checking customSecret annotations
				check := false
				for _, volume := range deployment.Spec.Template.Spec.Volumes {
					if volume.Secret != nil {
						if key == "cs-"+volume.Secret.SecretName {
							check = true
							deploymentAnnotation[key] = value
						}
					}
				}

				//check configMap Name in container
				for _, container := range deployment.Spec.Template.Spec.Containers {
					for _, env := range container.EnvFrom {
						if env.SecretRef != nil {
							if key == "cs-"+env.SecretRef.Name {
								check = true
								deploymentAnnotation[key] = value
							}
						}
					}
				}

				//check configMap Name in initContainer
				for _, initContainer := range deployment.Spec.Template.Spec.InitContainers {
					for _, env := range initContainer.EnvFrom {
						if env.SecretRef != nil {
							if key == "cs-"+env.SecretRef.Name {
								check = true
								deploymentAnnotation[key] = value
							}
						}
					}
				}

				if !check {
					removeAnnotation[key] = value
				}
			} else {
				deploymentAnnotation[key] = value
			}
		}
	}
	//it to handle the deployment in pod validation
	addnewAnnotation["config-sync-controller"] = "configurator"

	patch = append(patch, updateAnnotation(deploymentAnnotation, addnewAnnotation, removeAnnotation)...)
	return json.Marshal(patch)
}

func updateAnnotation(target map[string]string, added map[string]string, remove map[string]string) (patch []patchOperation) {
	//replace the target patch
	for tkey, tval := range target {
		// to handle forward slash in key name
		//Replace the forward slash (/) in kubernetes.io/ingress.class with ~1
		key := ""
		if strings.Contains(tkey, "/") {
			key = strings.Replace(tkey, "/", "~1", -1)
		} else {
			key = tkey
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  "/spec/template/metadata/annotations/" + key,
			Value: tval,
		})
	}

	//patch new annotation or replace the annotation
	//it check patch add operation first time
	checkAdd := false
	for key, value := range added {
		// to handle forward slash in key name
		//Replace the forward slash (/) in kubernetes.io/ingress.class with ~1
		akey := ""
		if strings.Contains(key, "/") {
			akey = strings.Replace(key, "/", "~1", -1)
		} else {
			akey = key
		}
		if !checkAdd {
			if target == nil || len(target) == 0 {
				checkAdd = true
				patch = append(patch, patchOperation{
					Op:   "add",
					Path: "/spec/template/metadata/annotations",
					Value: map[string]string{
						akey: value,
					},
				})
			} else {
				patch = append(patch, patchOperation{
					Op:    "replace",
					Path:  "/spec/template/metadata/annotations/" + akey,
					Value: value,
				})
			}
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/spec/template/metadata/annotations/" + akey,
				Value: value,
			})
		}

	}
	//remove unused ccm annotation
	for key, value := range remove {
		// to handle forward slash in key name
		//Replace the forward slash (/) in kubernetes.io/ingress.class with ~1
		akey := ""
		if strings.Contains(key, "/") {
			akey = strings.Replace(key, "/", "~1", -1)
		} else {
			akey = key
		}
		patch = append(patch, patchOperation{
			Op:    "remove",
			Path:  "/spec/template/metadata/annotations/" + akey,
			Value: value,
		})
	}
	return patch
}
