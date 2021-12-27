package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"time"

	customConfigMapv1alpha1 "github.com/gopaddle-io/configurator/apis/configurator.gopaddle.io/v1alpha1"
	customSecretv1alpha1 "github.com/gopaddle-io/configurator/apis/configurator.gopaddle.io/v1alpha1"
	client "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func initController() error {
	//getting cluster config for K8s
	var cfg *rest.Config
	var err error
	cfg, err = rest.InClusterConfig()
	//create clientset
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
	}

	//list all Namespaces
	namespaceList, err := clientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed on listing Namespace: %v", err.Error())
		return err
	}
	if namespaceList != nil {
		for _, ns := range namespaceList.Items {
			//get all deployment form the Namespace
			deploymentList, errs := clientSet.AppsV1().Deployments(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if errs != nil {
				klog.Errorf("Failed on listing deployment: %v", errs.Error())
				return errs
			}

			// check the deployment contain configMap/secret
			// if the deployment contain configMap/secret check the version of ccm and cs
			// version available add annotation if not create new version
			for _, deploy := range deploymentList.Items {

				//check the configMap/secret
				annotations := make(map[string]string)
				for _, volume := range deploy.Spec.Template.Spec.Volumes {

					if volume.ConfigMap != nil {
						if deploy.Spec.Template.Annotations["ccm-"+volume.ConfigMap.Name] == "" || len(deploy.Spec.Template.Annotations) == 0 {

							//get configMap
							configmap, e := clientSet.CoreV1().ConfigMaps(ns.Name).Get(context.TODO(), volume.ConfigMap.Name, metav1.GetOptions{})
							if e != nil {
								klog.Errorf("Failed on getting configmap: %v", e.Error())
								return e
							}

							if configmap.Annotations["currentCustomConfigMapVersion"] == "" || len(configmap.Annotations) == 0 {
								//create new ccm
								configuratorClientSet, err := client.NewForConfig(cfg)
								if err != nil {
									klog.Error("Error building example clientset: %s", err.Error())
									return err
								}
								ccm, version := newCustomConfigMap(configmap)
								_, er := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(ns.Name).Create(context.TODO(), ccm, metav1.CreateOptions{})
								if er != nil {
									return er
								}
								//update ccmVersion in configMap
								if len(configmap.Annotations) == 0 {
									confannotations := make(map[string]string)
									confannotations["currentCustomConfigMapVersion"] = version
									confannotations["customConfigMap-name"] = ccm.Name
									confannotations["updateMethod"] = "ignoreWhenShared"
									configmap.Annotations = confannotations
								} else {
									configmap.Annotations["currentCustomConfigMapVersion"] = version
									configmap.Annotations["customConfigMap-name"] = ccm.Name
									configmap.Annotations["updateMethod"] = "ignoreWhenShared"
								}

								_, errs := clientSet.CoreV1().ConfigMaps(ns.Name).Update(context.TODO(), configmap, metav1.UpdateOptions{})
								if errs != nil {
									klog.Error("configmap update failed", errs.Error())
									return errs
								}
								annotations["ccm-"+volume.ConfigMap.Name] = version
							}
						}
					} else if volume.Secret != nil {
						if deploy.Spec.Template.Annotations["cs-"+volume.Secret.SecretName] == "" || len(deploy.Spec.Template.Annotations) == 0 {
							//get secret
							secret, e := clientSet.CoreV1().Secrets(ns.Name).Get(context.TODO(), volume.Secret.SecretName, metav1.GetOptions{})
							if e != nil {
								klog.Errorf("Failed on getting configmap: %v", e.Error())
								return e
							}
							if secret.Annotations["currentCustomSecretVersion"] == "" || len(secret.Annotations) == 0 {
								configuratorClientSet, err := client.NewForConfig(cfg)
								if err != nil {
									klog.Error("Error building example clientset: %s", err.Error())
									return err
								}
								cs, version := newCustomSecret(secret)
								cs, er := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(ns.Name).Create(context.TODO(), cs, metav1.CreateOptions{})
								if er != nil {
									klog.Error("Error creating customConfigmap: %v", er.Error())
									return er
								}
								if len(secret.Annotations) == 0 {
									secretannotations := make(map[string]string)
									secretannotations["currentCustomSecretVersion"] = version
									secretannotations["customSecret-name"] = cs.Name
									secretannotations["updateMethod"] = "ignoreWhenShared"
									secret.Annotations = secretannotations
								} else {
									secret.Annotations["currentCustomSecretVersion"] = version
									secret.Annotations["customSecret-name"] = cs.Name
									secret.Annotations["updateMethod"] = "ignoreWhenShared"
								}
								_, errs := clientSet.CoreV1().Secrets(ns.Name).Update(context.TODO(), secret, metav1.UpdateOptions{})
								if errs != nil {
									klog.Error("secret update failed", errs.Error())
									return errs
								}
								annotations["cs-"+volume.Secret.SecretName] = version
							}
						}
					}
				}
				if len(deploy.Spec.Template.Annotations) == 0 {
					deploy.Spec.Template.Annotations = annotations
				} else {
					if len(annotations) != 0 {
						for key, value := range annotations {
							deploy.Spec.Template.Annotations[key] = value
						}
					}
				}

				//update deployment
				if len(annotations) != 0 {
					_, err := clientSet.AppsV1().Deployments(ns.Name).Update(context.TODO(), &deploy, metav1.UpdateOptions{})
					if err != nil {
						klog.Errorf("Failed on updating deployment with annotation: %v", err.Error())
						return err
					}
				}
			}

			//get all statefulset form the Namespace
			stsList, errs := clientSet.AppsV1().StatefulSets(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if errs != nil {
				klog.Errorf("Failed on listing statefulset: %v", errs.Error())
				return errs
			}

			// check the statefulset contain configMap/secret
			// if the statefulset contain configMap/secret check the version of ccm and cs
			// version available add annotation if not create new version
			for _, sts := range stsList.Items {

				stsAnnotation := make(map[string]string)
				for _, volume := range sts.Spec.Template.Spec.Volumes {
					if volume.ConfigMap != nil {
						if sts.Spec.Template.Annotations["ccm-"+volume.ConfigMap.Name] == "" || len(sts.Spec.Template.Annotations) == 0 {
							//get configMap
							configmap, e := clientSet.CoreV1().ConfigMaps(ns.Name).Get(context.TODO(), volume.ConfigMap.Name, metav1.GetOptions{})
							if e != nil {
								klog.Errorf("Failed on getting configmap: %v", e.Error())
								return e
							}

							if configmap.Annotations["currentCustomConfigMapVersion"] == "" || len(configmap.Annotations) == 0 {
								//create new ccm
								configuratorClientSet, err := client.NewForConfig(cfg)
								if err != nil {
									klog.Error("Error building example clientset: %s", err.Error())
									return err
								}
								ccm, version := newCustomConfigMap(configmap)
								_, er := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(ns.Name).Create(context.TODO(), ccm, metav1.CreateOptions{})
								if er != nil {
									return er
								}
								//update ccmVersion in configMap
								configmap.Annotations["currentCustomConfigMapVersion"] = version
								configmap.Annotations["customConfigMap-name"] = ccm.Name
								configmap.Annotations["updateMethod"] = "ignoreWhenShared"
								_, errs := clientSet.CoreV1().ConfigMaps(ns.Name).Update(context.TODO(), configmap, metav1.UpdateOptions{})
								if errs != nil {
									klog.Error("configmap update failed", errs.Error())
									return errs
								}
								stsAnnotation["ccm-"+volume.ConfigMap.Name] = version
							}
						}
					} else if volume.Secret != nil {
						if sts.Spec.Template.Annotations["cs-"+volume.Secret.SecretName] == "" || len(sts.Spec.Template.Annotations) == 0 {
							//get secret
							secret, e := clientSet.CoreV1().Secrets(ns.Name).Get(context.TODO(), volume.Secret.SecretName, metav1.GetOptions{})
							if e != nil {
								klog.Errorf("Failed on getting configmap: %v", e.Error())
								return e
							}
							if secret.Annotations["currentCustomSecretVersion"] == "" || len(secret.Annotations) == 0 {
								configuratorClientSet, err := client.NewForConfig(cfg)
								if err != nil {
									klog.Error("Error building example clientset: %s", err.Error())
									return err
								}
								cs, version := newCustomSecret(secret)
								cs, er := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(ns.Name).Create(context.TODO(), cs, metav1.CreateOptions{})
								if er != nil {
									klog.Error("Error creating customConfigmap: %v", er.Error())
									return er
								}

								secret.Annotations["currentCustomSecretVersion"] = version
								secret.Annotations["customSecret-name"] = cs.Name
								secret.Annotations["updateMethod"] = "ignoreWhenShared"
								_, errs := clientSet.CoreV1().Secrets(ns.Name).Update(context.TODO(), secret, metav1.UpdateOptions{})
								if errs != nil {
									klog.Error("secret update failed", errs.Error())
									return errs
								}
								stsAnnotation["cs-"+volume.Secret.SecretName] = version
							}
						}
					}
				}

				if len(sts.Spec.Template.Annotations) == 0 {
					sts.Spec.Template.Annotations = stsAnnotation
				} else {
					if len(stsAnnotation) != 0 {
						for key, value := range stsAnnotation {
							sts.Spec.Template.Annotations[key] = value
						}
					}
				}

				//update statefulset
				if len(stsAnnotation) != 0 {
					_, err := clientSet.AppsV1().StatefulSets(ns.Name).Update(context.TODO(), &sts, metav1.UpdateOptions{})
					if err != nil {
						klog.Errorf("Failed on updating statefulset with annotation: %v", err.Error())
						return err
					}
				}

			}

		}
	}
	return nil
}

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
