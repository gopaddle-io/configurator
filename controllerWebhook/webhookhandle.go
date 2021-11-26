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
	clientset "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	v1 "k8s.io/api/admission/v1"
	appsV1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type WebhookServer struct {
	Server *http.Server
}

// Webhook Server parameters
type WhSvrParameters struct {
	CertFile string // path to the x509 certificate for https
	KeyFile  string // path to the x509 private key matching `CertFile`
}

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
		admissionResponse = mutate(&ar)
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
func mutate(ar *v1.AdmissionReview) *v1.AdmissionResponse {
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
						configMap.Annotations["deployments"] = deployment.Name
					}
				}
				configMap, errs := clientSet.CoreV1().ConfigMaps(deployment.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
				if errs != nil {
					return nil, errs
				}
				addnewAnnotation["ccm-"+configMap.Name] = configMap.Annotations["currentCustomConfigMapVersion"]
			}
		}
	}
	//remove annotation in deployment if that configmap name is not there
	deploymentAnnotation := make(map[string]string)
	removeAnnotation := make(map[string]string)
	if len(deployment.Spec.Template.Annotations) != 0 {
		for key, value := range deployment.Spec.Template.Annotations {
			if strings.Contains(key, "ccm-") {
				check := false
				for _, volume := range deployment.Spec.Template.Spec.Volumes {
					if volume.ConfigMap != nil {
						if key == "ccm-"+volume.ConfigMap.Name {
							check = true
							deploymentAnnotation[key] = value
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
	patch = append(patch, updateAnnotation(deploymentAnnotation, addnewAnnotation, removeAnnotation)...)
	return json.Marshal(patch)
}

func updateAnnotation(target map[string]string, added map[string]string, remove map[string]string) (patch []patchOperation) {
	//replace the target patch
	for tkey, tval := range target {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  "/spec/template/metadata/annotations/" + tkey,
			Value: tval,
		})
	}

	//patch new annotation or replace the annotation
	for key, value := range added {
		// if target == nil || target[key] == "" {
		// 	target = map[string]string{}
		// 	patch = append(patch, patchOperation{
		// 		Op:   "add",
		// 		Path: "/spec/template/metadata/annotations",
		// 		Value: map[string]string{
		// 			key: value,
		// 		},
		// 	})
		//} else {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  "/spec/template/metadata/annotations/" + key,
			Value: value,
		})
		//}
	}
	//remove unused ccm annotation
	for key, value := range remove {
		patch = append(patch, patchOperation{
			Op:    "remove",
			Path:  "/spec/template/metadata/annotations/" + key,
			Value: value,
		})
	}
	return patch
}

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
