package watcher

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
)

const (
	path = "labelconfig/"
)

func ConfigRollingUpdate(rw http.ResponseWriter, req *http.Request) {
	clientConfig := Config{}
	clientConfigFile := Config{}
	data, e := ioutil.ReadAll(req.Body)
	if e != nil {
		rw.WriteHeader(500)
		return
	}
	e = json.Unmarshal(data, &clientConfig)
	if e != nil {
		log.Println("Failed on unmarshalling the request ", e.Error(), time.Now().UTC())
		rw.WriteHeader(500)
		rw.Write([]byte(e.Error()))
		return
	}

	fileNotfound := false
	filePath := path + "labelconfig.json"
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		fileNotfound = true
		if errs := os.MkdirAll(path, 0755); errs != nil {
			log.Println("Failed to create directory: ", errs)
			rw.WriteHeader(500)
			rw.Write([]byte("failed on creating file"))
			return
		}

	}
	labelFile, errs := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if errs != nil {
		log.Println("Failed to create file: ", errs.Error())
		rw.WriteHeader(500)
		rw.Write([]byte(fmt.Sprintf("failed on opening file", err.Error())))
		return
	}
	if fileNotfound {
		n2, errs := labelFile.Write(data)
		if errs != nil {
			log.Println("Failed to write in log file ", n2, errs, time.Now().UTC())
			rw.WriteHeader(500)
			rw.Write([]byte("Failed on writing data into file"))
			return
		}
	} else {
		e = json.Unmarshal(file, &clientConfigFile)
		if e != nil {
			log.Println("Failed on unmarshalling the file content ", e.Error(), time.Now().UTC())
			rw.WriteHeader(500)
			rw.Write([]byte(e.Error()))
			return
		}
		for _, label := range clientConfig.Labels {
			check := false
			for _, fileLabel := range clientConfigFile.Labels {
				fileConfigLabel := "name=" + fileLabel.ConfigMap
				reqConfigLabel := "name=" + label.ConfigMap
				if fileConfigLabel == reqConfigLabel && fileLabel.NameSpace == label.NameSpace {
					check = true
				}
			}
			if !check {
				clientConfigFile.Labels = append(clientConfigFile.Labels, label)
			}
		}
		//removeing configFile
		os.Remove(filePath)
		//create a new file add a file content
		labelFile, errs := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if errs != nil {
			log.Println("Failed to create file: ", errs, time.Now().UTC())
			rw.WriteHeader(500)
			rw.Write([]byte("Failed on opening file"))
			return
		}
		fileData, _ := json.Marshal(clientConfigFile)
		n2, errs := labelFile.Write(fileData)
		if errs != nil {
			log.Println("Failed to write content to log file ", n2, errs, time.Now().UTC())
			rw.WriteHeader(500)
			rw.Write([]byte("Failed on writing data into file"))
			return
		}

	}
	for _, label := range clientConfig.Labels {
		log.Println(fmt.Sprintf("Started watcher for configmap label '%s' in nameSpace '%s'", label.ConfigMap, label.NameSpace), time.Now().UTC())
		go StartWatcher(label)
	}
	rw.WriteHeader(200)
	rw.Write([]byte("watcher started successfully"))
	return
}

func StartWatcher(label ConfigLabel) {
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
	configLabel := "name=" + label.ConfigMap
	listOpts.LabelSelector = configLabel

	watch, er := clientSet.CoreV1().ConfigMaps(label.NameSpace).Watch(listOpts)
	if er != nil {
		log.Println("failed on watching configmap label '%s'", label.ConfigMap)
		return
	}
	previousConfigName := ""
	previosuCreatedTime := ""
	layout := "2006-01-02T15:04:05Z07:00"
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

			if previousConfigName == "" {
				previousConfigName = name
				previosuCreatedTime = metadata["creationTimestamp"].(string)
			}
			pretime, _ := time.Parse(layout, previosuCreatedTime)
			configTime, _ := time.Parse(layout, metadata["creationTimestamp"].(string))
			if pretime.Equal(configTime) {
				continue
			} else if configTime.After(pretime) {
				previosuCreatedTime = metadata["creationTimestamp"].(string)
				labelStr := SplitStr(previousConfigName)
				var opts metav1.ListOptions
				opts.LabelSelector = labelStr
				deploylist, er := clientSet.AppsV1().Deployments(label.NameSpace).List(opts)
				if er != nil {
					log.Println("Failed on getting deployment list based on label", er, time.Now().UTC())
				}
				for _, deployment := range deploylist.Items {
					volumes := deployment.Spec.Template.Spec.Volumes
					for index, volume := range volumes {
						if volume.ConfigMap.Name == previousConfigName {
							volume.ConfigMap.Name = name
							volumes[index] = volume
							deployment.Spec.Template.Spec.Volumes = volumes
							newLabel := SplitStr(name)
							s := strings.Split(newLabel, "=")
							deployment.Labels[s[0]] = s[1]
							annotations := make(map[string]string)
							annotations["kubernetes.io/change-cause"] = "configmap updated to " + name
							deployment.Annotations = annotations
							_, err := clientSet.AppsV1().Deployments(label.NameSpace).Update(&deployment)
							if err != nil {
								log.Fatal(fmt.Sprintf("Failed on updating deployment '%s'", deployment.Name), "Error", err.Error(), time.Now().UTC())
							}
							log.Println(fmt.Sprintf("deployment '%s' updated with config '%s'", deployment.Name, name), time.Now().UTC())
							//previousConfigName = name
						}
					}
				}
				previousConfigName = name
			}
		}
	}

}


// Start the watcher if any previous labels were present
func TriggerWatcher() error {
	clientConfig := Config{}
	//reading file
	filePath := path + "labelconfig.json"
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Println("Configuration file not found", err.Error(), time.Now().UTC())
		return err
	}
	e := json.Unmarshal(file, &clientConfig)
	if e != nil {
		log.Println("failed on unmarshal", e.Error(), time.Now().UTC())
		return e
	}

	for _, label := range clientConfig.Labels {
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

func PurgeConfig() {
	log.Println("purge unused config started", time.Now().UTC())
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

	clientConfig := Config{}
	//reading file
	filePath := path + "labelconfig.json"
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Println("Configuration file not found", time.Now().UTC())
		return
	}
	e := json.Unmarshal(file, &clientConfig)
	if e != nil {
		log.Println("Failed to unmarshal", e.Error(), time.Now().UTC())
		return
	}
	for _, label := range clientConfig.Labels {
		var listOpts metav1.ListOptions
		configLabel := "name=" + label.ConfigMap
		listOpts.LabelSelector = configLabel
		//getting all configmap with specific label
		listConfig, er := clientSet.CoreV1().ConfigMaps(label.NameSpace).List(listOpts)
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
			deploylist, er := clientSet.AppsV1().Deployments(label.NameSpace).List(opts)
			if er != nil {
				log.Println(fmt.Sprintf("Failed on getting deployment list based on label '%s'", label.ConfigMap), er, time.Now().UTC())
			}
			for _, deployment := range deploylist.Items {
				
				_, allOldRSs, newRS, err := deploymentutil.GetAllReplicaSets(&deployment, clientSet.AppsV1())
				if err != nil {
					log.Println("Failed to retrieve replica sets from deployment %v", err, time.Now().UTC())
				}
				allRSs := allOldRSs
				if newRS != nil {
					allRSs = append(allRSs, newRS)
				}
				for _, rs := range allRSs {
					volumes := rs.Spec.Template.Spec.Volumes
					for _, volume := range volumes {
						if volume.ConfigMap.Name == configName {
							configCheck = true
						}
					}
				}
			}
			if len(deploylist.Items) != 0 {
				if !configCheck {
					//purging configmap
					delOpts := metav1.DeleteOptions{}
					er := clientSet.CoreV1().ConfigMaps(label.NameSpace).Delete(configName, &delOpts)
					if er != nil {
						log.Println(fmt.Sprintf("Failed on parge configmap '%s'", configName), "Error", er, time.Now().UTC())
					} else {
						log.Println(fmt.Sprintf("config purged successfully '%s'", configName), time.Now().UTC())
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
		cron.Cron.AddFunc("@every 2m", PurgeConfig)
		cron.Cron.Start()
	}()
}
