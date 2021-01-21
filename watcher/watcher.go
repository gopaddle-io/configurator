package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/robfig/cron"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	path = "labelconfig/"
)

func StartWatcher(label WatcherLabel) {
	//creating clientset
	args := os.Args[1:]
	if len(args) < 1 {
		log.Panicln("Kubernetes Client Config is not provided,\n\t")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", args[0])
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s", err.Error(), time.Now().UTC())
		return
	}
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
		return
	}

	var listOpts metav1.ListOptions

	listOpts.Watch = true
	if label.ConfigMap != "" {
		configLabel := "name=" + label.ConfigMap
		listOpts.LabelSelector = configLabel
	} else if label.Secret != "" {
		secretLabel := "name=" + label.Secret
		listOpts.LabelSelector = secretLabel
	}
	var watch watch.Interface
	var er error
	if label.ConfigMap != "" {
		for {
			watch, er = clientSet.CoreV1().ConfigMaps(label.NameSpace).Watch(context.TODO(), listOpts)
			if er != nil {
				log.Println("failed on watching configmap label '%s'", label.ConfigMap)
				log.Println("error in wtach --->", er.Error())
				continue
			} else {
				break
			}
		}
	} else if label.Secret != "" {
		for {
			watch, er = clientSet.CoreV1().Secrets(label.NameSpace).Watch(context.TODO(), listOpts)
			if er != nil {
				log.Println("failed on watching configmap label '%s'", label.Secret)
				log.Println("error in wtach --->", er.Error())
				continue
			} else {
				break
			}
		}
	}
	previousName := ""
	previousCreatedTime := ""
	layout := "2006-01-02T15:04:05Z"
	for {
		result := <-watch.ResultChan()
		if result.Type == "ADDED" {
			data, errs := json.Marshal(result.Object)
			if errs != nil {
				log.Println("Failed to marshal", errs, time.Now().UTC())
			}
			var f interface{}
			if err := json.Unmarshal(data, &f); err != nil {
				log.Println("Failed to unmarshal", err, time.Now().UTC())
			}
			object := f.(map[string]interface{})
			//geting metadata
			metadata := object["metadata"].(map[string]interface{})
			name := metadata["name"].(string)

			if previousName == "" {
				previousName = name
				previousCreatedTime = metadata["creationTimestamp"].(string)
			}
			pretime, err := time.Parse(layout, previousCreatedTime)
			if err != nil {
				log.Println("failed on parse time", err, time.Now().UTC())
			}
			currenCreatedTime, er := time.Parse(layout, metadata["creationTimestamp"].(string))
			if er != nil {
				log.Println("failed on parse time", er, time.Now().UTC())
			}
			if pretime.Equal(currenCreatedTime) {
				continue
			} else if currenCreatedTime.After(pretime) {
				previousCreatedTime = metadata["creationTimestamp"].(string)
				labelStr := SplitStr(previousName)
				var opts metav1.ListOptions
				opts.LabelSelector = labelStr
				//deployment update
				deploylist, er := clientSet.AppsV1().Deployments(label.NameSpace).List(context.TODO(), opts)
				if er != nil {
					log.Println("Failed on getting deployment list based on label", er, time.Now().UTC())
				}
				if len(deploylist.Items) != 0 {
					for _, deployment := range deploylist.Items {
						//volume configmap
						volumes := deployment.Spec.Template.Spec.Volumes
						for index, volume := range volumes {
							if label.ConfigMap != "" {
								if volume.ConfigMap != nil {
									if volume.ConfigMap.Name == previousName {
										volume.ConfigMap.Name = name
										volumes[index] = volume
										deployment.Spec.Template.Spec.Volumes = volumes
										newLabel := SplitStr(name)
										s := strings.Split(newLabel, "=")
										deployment.Labels[s[0]] = s[1]
										annotations := make(map[string]string)
										annotations["kubernetes.io/change-cause"] = "configmap updated to " + name
										deployment.Annotations = annotations
									}
								}
							} else if label.Secret != "" {
								if volume.Secret != nil {
									if volume.Secret.SecretName == previousName {
										volume.Secret.SecretName = name
										volumes[index] = volume
										deployment.Spec.Template.Spec.Volumes = volumes
										newLabel := SplitStr(name)
										s := strings.Split(newLabel, "=")
										deployment.Labels[s[0]] = s[1]
										annotations := make(map[string]string)
										annotations["kubernetes.io/change-cause"] = "Secret updated to " + name
										deployment.Annotations = annotations
									}
								}
							}
						}
						//container env configmap update
						deploymentContainer := deployment.Spec.Template.Spec.Containers
						for index, container := range deployment.Spec.Template.Spec.Containers {
							for ind, envref := range container.EnvFrom {
								if label.ConfigMap != "" {
									if envref.ConfigMapRef != nil {
										if envref.ConfigMapRef.Name == previousName {
											envref.ConfigMapRef.Name = name
											deploymentContainer[index].EnvFrom[ind] = envref
											deployment.Spec.Template.Spec.Containers = deploymentContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											deployment.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "container '" + container.Name + "'env configmap updated to " + name
											deployment.Annotations = annotations
										}
									}
								} else if label.Secret != "" {
									if envref.SecretRef != nil {
										if envref.SecretRef.Name == previousName {
											envref.SecretRef.Name = name
											deploymentContainer[index].EnvFrom[ind] = envref
											deployment.Spec.Template.Spec.Containers = deploymentContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											deployment.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "container '" + container.Name + "'env secret updated to " + name
											deployment.Annotations = annotations
										}
									}
								}
							}
						}
						//init container env configmap update
						deployInitContainer := deployment.Spec.Template.Spec.InitContainers
						for index, initContainer := range deployment.Spec.Template.Spec.InitContainers {
							for ind, envref := range initContainer.EnvFrom {
								if label.ConfigMap != "" {
									if envref.ConfigMapRef != nil {
										if envref.ConfigMapRef.Name == previousName {
											envref.ConfigMapRef.Name = name
											deployInitContainer[index].EnvFrom[ind] = envref
											deployment.Spec.Template.Spec.InitContainers = deployInitContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											deployment.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "initContainer '" + initContainer.Name + "' env configmap updated to " + name
											deployment.Annotations = annotations
										}
									}
								} else if label.Secret != "" {
									if envref.SecretRef != nil {
										if envref.SecretRef.Name == previousName {
											envref.SecretRef.Name = name
											deployInitContainer[index].EnvFrom[ind] = envref
											deployment.Spec.Template.Spec.InitContainers = deployInitContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											deployment.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "initContainer '" + initContainer.Name + "' env secret updated to " + name
											deployment.Annotations = annotations
										}
									}
								}
							}
						}
						//update deployment template
						updOpts := metav1.UpdateOptions{}
						_, err := clientSet.AppsV1().Deployments(label.NameSpace).Update(context.TODO(), &deployment, updOpts)
						if err != nil {
							log.Println(fmt.Sprintf("Failed on updating deployment '%s'", deployment.Name), "Error ", err.Error(), time.Now().UTC())
						} else {
							log.Println(fmt.Sprintf("deployment '%s' updated with config '%s'", deployment.Name, name), time.Now().UTC())
						}
					}
				}
				//statefulser update
				stslist, er := clientSet.AppsV1().StatefulSets(label.NameSpace).List(context.TODO(), opts)
				if er != nil {
					log.Println("Failed on getting statefulset list based on label", er, time.Now().UTC())
				}
				if len(stslist.Items) != 0 {
					for _, sts := range stslist.Items {
						//volume configmap
						volumes := sts.Spec.Template.Spec.Volumes
						for index, volume := range volumes {
							if label.ConfigMap != "" {
								if volume.ConfigMap != nil {
									if volume.ConfigMap.Name == previousName {
										volume.ConfigMap.Name = name
										volumes[index] = volume
										sts.Spec.Template.Spec.Volumes = volumes
										newLabel := SplitStr(name)
										s := strings.Split(newLabel, "=")
										sts.Labels[s[0]] = s[1]
										annotations := make(map[string]string)
										annotations["kubernetes.io/change-cause"] = "configmap updated to " + name
										sts.Annotations = annotations
									}
								}
							} else if label.Secret != "" {
								if volume.Secret != nil {
									if volume.Secret.SecretName == previousName {
										volume.Secret.SecretName = name
										volumes[index] = volume
										sts.Spec.Template.Spec.Volumes = volumes
										newLabel := SplitStr(name)
										s := strings.Split(newLabel, "=")
										sts.Labels[s[0]] = s[1]
										annotations := make(map[string]string)
										annotations["kubernetes.io/change-cause"] = "secret updated to " + name
										sts.Annotations = annotations
									}
								}
							}
						}
						//container env configmap update
						stsContainer := sts.Spec.Template.Spec.Containers
						for index, container := range sts.Spec.Template.Spec.Containers {
							for ind, envref := range container.EnvFrom {
								if label.ConfigMap != "" {
									if envref.ConfigMapRef != nil {
										if envref.ConfigMapRef.Name == previousName {
											envref.ConfigMapRef.Name = name
											stsContainer[index].EnvFrom[ind] = envref
											sts.Spec.Template.Spec.Containers = stsContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											sts.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "container '" + container.Name + "'env configmap updated to " + name
											sts.Annotations = annotations
										}
									}
								} else if label.Secret != "" {
									if envref.SecretRef != nil {
										if envref.SecretRef.Name == previousName {
											envref.SecretRef.Name = name
											stsContainer[index].EnvFrom[ind] = envref
											sts.Spec.Template.Spec.Containers = stsContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											sts.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "container '" + container.Name + "'env secret updated to " + name
											sts.Annotations = annotations
										}
									}
								}
							}
						}
						//init container env configmap update
						stsInitContainer := sts.Spec.Template.Spec.InitContainers
						for index, initContainer := range sts.Spec.Template.Spec.InitContainers {
							for ind, envref := range initContainer.EnvFrom {
								if label.ConfigMap != "" {
									if envref.ConfigMapRef != nil {
										if envref.ConfigMapRef.Name == previousName {
											envref.ConfigMapRef.Name = name
											stsInitContainer[index].EnvFrom[ind] = envref
											sts.Spec.Template.Spec.InitContainers = stsInitContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											sts.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "initContainer '" + initContainer.Name + "' env configmap updated to " + name
											sts.Annotations = annotations
										}
									}
								} else if label.Secret != "" {
									if envref.SecretRef != nil {

										if envref.SecretRef.Name == previousName {
											envref.SecretRef.Name = name
											stsInitContainer[index].EnvFrom[ind] = envref
											sts.Spec.Template.Spec.InitContainers = stsInitContainer
											newLabel := SplitStr(name)
											s := strings.Split(newLabel, "=")
											sts.Labels[s[0]] = s[1]
											annotations := make(map[string]string)
											annotations["kubernetes.io/change-cause"] = "initContainer '" + initContainer.Name + "' env secret updated to " + name
											sts.Annotations = annotations
										}
									}
								}
							}
						}
						//update deployment template
						updOpts := metav1.UpdateOptions{}
						_, err := clientSet.AppsV1().StatefulSets(label.NameSpace).Update(context.TODO(), &sts, updOpts)
						if err != nil {
							log.Println(fmt.Sprintf("Failed on updating statefulset '%s'", sts.Name), "Error ", err.Error(), time.Now().UTC())
						} else {
							log.Println(fmt.Sprintf("statefulset '%s' updated with config '%s'", sts.Name, name), time.Now().UTC())
						}
					}
				}
				previousName = name
			}
		}
	}

}

// Start the watcher if any previous labels were present
func TriggerWatcher() error {
	clientWatcher := Watcher{}
	//reading file
	filePath := path + "labelconfig.json"
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Println("Configuration file not found", err.Error(), time.Now().UTC())
		return err
	}
	e := json.Unmarshal(file, &clientWatcher)
	if e != nil {
		log.Println("failed on unmarshal", e.Error(), time.Now().UTC())
		return e
	}

	for _, label := range clientWatcher.Labels {
		log.Println(fmt.Sprintf("Start watcher for configmap label '%s' in nameSpace '%s'", label.ConfigMap, label.NameSpace), time.Now().UTC())
		go StartWatcher(label)
	}
	return nil

}

func SplitStr(str string) string {
	var labelStr string
	splitstr := strings.Split(str, "-")
	for i := 0; i < len(splitstr); i++ {
		if i == 0 {
			labelStr = labelStr + splitstr[i]
		} else if i == len(splitstr)-1 {
			labelStr = labelStr + "=" + splitstr[i]
		} else {
			labelStr = labelStr + "-" + splitstr[i]
		}
	}
	return labelStr
}

//purge unused configmaps and secret
func PurgeConfigAndSecret() {
	log.Println("purge unused configmap and secret started", time.Now().UTC())
	args := os.Args[1:]
	if len(args) < 1 {
		log.Panicln("Kubernetes Client Configuration is not provided,\n\t")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", args[0])
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s", err.Error())
	}
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	clientWatcher := Watcher{}
	//reading file
	filePath := path + "labelconfig.json"
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Println("Configuration file not found", time.Now().UTC())
		return
	}
	e := json.Unmarshal(file, &clientWatcher)
	if e != nil {
		log.Println("Failed to unmarshal", e.Error(), time.Now().UTC())
		return
	}
	for _, label := range clientWatcher.Labels {
		var listOpts metav1.ListOptions
		if label.ConfigMap != "" {
			configLabel := "name=" + label.ConfigMap
			listOpts.LabelSelector = configLabel
			//getting all configmap with specific label
			listConfig, er := clientSet.CoreV1().ConfigMaps(label.NameSpace).List(context.TODO(), listOpts)
			if er != nil {
				log.Println("Failed on watching configmap label '%s'", label.ConfigMap, time.Now().UTC())
				continue
			}

			for _, config := range listConfig.Items {
				configName := config.Name
				configCheck := false
				var opts metav1.ListOptions
				confLabel := SplitStr(configName)
				s := strings.Split(confLabel, "=")
				opts.LabelSelector = s[0]
				//getting all deployment with specific label
				deploylist, er := clientSet.AppsV1().Deployments(label.NameSpace).List(context.TODO(), opts)
				if er != nil {
					log.Println(fmt.Sprintf("Failed on getting deployment list based on label '%s'", label.ConfigMap), er, time.Now().UTC())
				}
				for _, deployment := range deploylist.Items {
					selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
					if err != nil {
						log.Println("Failed get selector from deployment %v", err, time.Now().UTC())
					}
					options := metav1.ListOptions{LabelSelector: selector.String()}
					allRSs, err := clientSet.AppsV1().ReplicaSets(label.NameSpace).List(context.TODO(), options)
					if err != nil {
						log.Println(fmt.Sprintf("Failed on getting deployment list based on label '%s'", label.ConfigMap), err, time.Now().UTC())
					}
					for _, rs := range allRSs.Items {
						volumes := rs.Spec.Template.Spec.Volumes
						for _, volume := range volumes {
							if volume.ConfigMap.Name == configName {
								configCheck = true
							}
						}
						for _, container := range rs.Spec.Template.Spec.Containers {
							for _, env := range container.EnvFrom {
								if env.ConfigMapRef.Name == configName {
									configCheck = true
								}
							}
						}
						//initContainer unused config purge part
						for _, initContainer := range rs.Spec.Template.Spec.InitContainers {
							for _, env := range initContainer.EnvFrom {
								if env.ConfigMapRef.Name == configName {
									configCheck = true
								}
							}
						}
					}
				}
				//getting all sts with specific label
				stslist, er := clientSet.AppsV1().StatefulSets(label.NameSpace).List(context.TODO(), opts)
				if er != nil {
					log.Println(fmt.Sprintf("Failed on getting statefulset list based on label '%s'", label.ConfigMap), er, time.Now().UTC())
				}
				for _, sts := range stslist.Items {
					selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
					if err != nil {
						log.Println("Failed get selector from deployment %v", err, time.Now().UTC())
					}
					options := metav1.ListOptions{LabelSelector: selector.String()}
					allRevisions, err := clientSet.AppsV1().ControllerRevisions(label.NameSpace).List(context.TODO(), options)
					if err != nil {
						log.Println(fmt.Sprintf("Failed on getting sts list based on label '%s'", label.ConfigMap), err, time.Now().UTC())
					}
					for _, rs := range allRevisions.Items {
						stsRevision := appsv1.StatefulSet{}
						er := json.Unmarshal(rs.Data.Raw, &stsRevision)
						if er != nil {
							log.Println("failed on unmarshal", er.Error(), time.Now().UTC())
						}
						volumes := stsRevision.Spec.Template.Spec.Volumes
						for _, volume := range volumes {
							if volume.ConfigMap != nil {
								if volume.ConfigMap.Name == configName {
									configCheck = true
								}
							}
						}
						for _, container := range stsRevision.Spec.Template.Spec.Containers {
							for _, env := range container.EnvFrom {
								if env.ConfigMapRef != nil {
									if env.ConfigMapRef.Name == configName {
										configCheck = true
									}
								}
							}
						}
						//initContainer unused config purge part
						for _, initContainer := range stsRevision.Spec.Template.Spec.InitContainers {
							for _, env := range initContainer.EnvFrom {
								if env.ConfigMapRef != nil {
									if env.ConfigMapRef.Name == configName {
										configCheck = true
									}
								}
							}
						}
					}
				}
				if len(deploylist.Items) != 0 || len(stslist.Items) != 0 {
					if !configCheck {
						//purging configmap
						delOpts := metav1.DeleteOptions{}
						er := clientSet.CoreV1().ConfigMaps(label.NameSpace).Delete(context.TODO(), configName, delOpts)
						if er != nil {
							log.Println(fmt.Sprintf("Failed on parge configmap '%s'", configName), "Error", er, time.Now().UTC())
						} else {
							log.Println(fmt.Sprintf("config purged successfully '%s'", configName), time.Now().UTC())
						}
					}
				}

			}
		} else if label.Secret != "" {
			secretLabel := "name=" + label.Secret
			listOpts.LabelSelector = secretLabel
			listSecret, er := clientSet.CoreV1().Secrets(label.NameSpace).List(context.TODO(), listOpts)
			if er != nil {
				log.Println("Failed on watching configmap label '%s'", label.ConfigMap, time.Now().UTC())
				continue
			}
			for _, secret := range listSecret.Items {
				secretName := secret.Name
				secretCheck := false
				var opts metav1.ListOptions
				secretLabel := SplitStr(secretName)
				s := strings.Split(secretLabel, "=")
				opts.LabelSelector = s[0]
				//getting all deployment with specific label
				deploylist, er := clientSet.AppsV1().Deployments(label.NameSpace).List(context.TODO(), opts)
				if er != nil {
					log.Println(fmt.Sprintf("Failed on getting deployment list based on label '%s'", label.Secret), er, time.Now().UTC())
				}
				for _, deployment := range deploylist.Items {
					selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
					if err != nil {
						log.Println("Failed get selector from deployment %v", err, time.Now().UTC())
					}
					options := metav1.ListOptions{LabelSelector: selector.String()}
					allRSs, err := clientSet.AppsV1().ReplicaSets(label.NameSpace).List(context.TODO(), options)
					if err != nil {
						log.Println(fmt.Sprintf("Failed on getting deployment list based on label '%s'", label.ConfigMap), err, time.Now().UTC())
					}
					for _, rs := range allRSs.Items {
						volumes := rs.Spec.Template.Spec.Volumes
						for _, volume := range volumes {
							if volume.Secret != nil {
								if volume.Secret.SecretName == secretName {
									secretCheck = true
								}
							}
						}
						for _, container := range rs.Spec.Template.Spec.Containers {
							for _, env := range container.EnvFrom {
								if env.SecretRef != nil {
									if env.SecretRef.Name == secretName {
										secretCheck = true
									}
								}
							}
						}
						//initContainer unused config purge part
						for _, initContainer := range rs.Spec.Template.Spec.InitContainers {
							for _, env := range initContainer.EnvFrom {
								if env.SecretRef != nil {
									if env.SecretRef.Name == secretName {
										secretCheck = true
									}
								}
							}
						}
					}
				}
				//getting all sts with specific label
				stslist, er := clientSet.AppsV1().StatefulSets(label.NameSpace).List(context.TODO(), opts)
				if er != nil {
					log.Println(fmt.Sprintf("Failed on getting statefulset list based on label '%s'", label.ConfigMap), er, time.Now().UTC())
				}
				for _, sts := range stslist.Items {
					selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
					if err != nil {
						log.Println("Failed get selector from deployment %v", err, time.Now().UTC())
					}
					options := metav1.ListOptions{LabelSelector: selector.String()}
					allRevisions, err := clientSet.AppsV1().ControllerRevisions(label.NameSpace).List(context.TODO(), options)
					if err != nil {
						log.Println(fmt.Sprintf("Failed on getting sts list based on label '%s'", label.ConfigMap), err, time.Now().UTC())
					}
					for _, rs := range allRevisions.Items {
						stsRevision := appsv1.StatefulSet{}
						er := json.Unmarshal(rs.Data.Raw, &stsRevision)
						if er != nil {
							log.Println("failed on unmarshal", er.Error(), time.Now().UTC())
						}
						volumes := stsRevision.Spec.Template.Spec.Volumes
						for _, volume := range volumes {
							if volume.Secret.SecretName == secretName {
								secretCheck = true
							}
						}
						for _, container := range stsRevision.Spec.Template.Spec.Containers {
							for _, env := range container.EnvFrom {
								if env.SecretRef.Name == secretName {
									secretCheck = true
								}
							}
						}
						//initContainer unused config purge part
						for _, initContainer := range stsRevision.Spec.Template.Spec.InitContainers {
							for _, env := range initContainer.EnvFrom {
								if env.SecretRef.Name == secretName {
									secretCheck = true
								}
							}
						}
					}
				}
				if len(deploylist.Items) != 0 || len(stslist.Items) != 0 {
					if !secretCheck {
						//purging configmap
						delOpts := metav1.DeleteOptions{}
						er := clientSet.CoreV1().Secrets(label.NameSpace).Delete(context.TODO(), secretName, delOpts)
						if er != nil {
							log.Println(fmt.Sprintf("Failed on parge configmap '%s'", secretName), "Error", er, time.Now().UTC())
						} else {
							log.Println(fmt.Sprintf("config purged successfully '%s'", secretName), time.Now().UTC())
						}
					}
				}

			}
		}
	}

}

//trigger purge configmap every 5 mins
func PurgeJob() {
	cron := CornJob{Cron: cron.New()}
	go func() {
		cron.Cron.AddFunc("@every 5m", PurgeConfigAndSecret)
		cron.Cron.Start()
	}()
}

//store label in file
func StoreLabel(clientWatcher Watcher) error {
	clientWatcherFile := Watcher{}
	fileNotfound := false
	filePath := path + "labelconfig.json"
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		fileNotfound = true
		if errs := os.MkdirAll(path, 0755); errs != nil {
			log.Println("Failed to create directory: ", errs)
			return errs
		}

	}
	labelFile, errs := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if errs != nil {
		log.Println("Failed to create file: ", errs.Error())
		return errs
	}
	if fileNotfound {
		data, e := json.Marshal(clientWatcher)
		if e != nil {
			fmt.Errorf("Failed on unmarshalling the request ", e.Error(), time.Now().UTC())
			return e
		}
		n2, errs := labelFile.Write(data)
		if errs != nil {
			log.Println("Failed to write in log file ", n2, errs, time.Now().UTC())
			return errs
		}
	} else {
		fi, _ := os.Stat(filePath)
		if fi.Size() != 0 {
			e := json.Unmarshal(file, &clientWatcherFile)
			if e != nil {
				log.Println("Failed on unmarshalling the file content ", e.Error(), time.Now().UTC())
				return e
			}
		}
		for _, label := range clientWatcher.Labels {
			check := false
			for _, fileLabel := range clientWatcherFile.Labels {
				fileConfigLabel := "name=" + fileLabel.ConfigMap
				reqConfigLabel := "name=" + label.ConfigMap
				if fileConfigLabel == reqConfigLabel && fileLabel.NameSpace == label.NameSpace {
					check = true
				}
			}
			if !check {
				clientWatcherFile.Labels = append(clientWatcherFile.Labels, label)
			}
		}
		//removeing configFile
		os.Remove(filePath)
		//create a new file add a file content
		labelFile, errs := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if errs != nil {
			log.Println("Failed to create file: ", errs, time.Now().UTC())
			return errs
		}
		fileData, _ := json.Marshal(clientWatcherFile)
		n2, errs := labelFile.Write(fileData)
		if errs != nil {
			log.Println("Failed to write content to log file ", n2, errs, time.Now().UTC())
			return errs
		}

	}
	return nil
}
