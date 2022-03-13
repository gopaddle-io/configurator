package configuratorgopaddleio

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	client "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	"github.com/robfig/cron"
	appsV1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CornJob struct {
	Cron *cron.Cron
}

//trigger purge every 5 mins
func PurgeJob() {
	cron := CornJob{Cron: cron.New()}
	go func() {
		cron.Cron.AddFunc("@every 15m", func() {
			PurgeCCMAndCS()
		})
		cron.Cron.Start()
	}()
}

//it remove unused customConfigMap and customSecret
func PurgeCCMAndCS() {
	var cfg *rest.Config
	var err error
	cfg, err = rest.InClusterConfig()
	//create clientset
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Error("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
	}

	configuratorClientSet, err := client.NewForConfig(cfg)
	if err != nil {
		klog.Error("Error building example clientset: %s", err.Error())
	}

	//list all namespace
	nsList, err := clientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed on listing Namespace: %v", err.Error())
	}
	for _, ns := range nsList.Items {
		//list all customConfigMap
		ccmList, errs := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if errs != nil {
			klog.Errorf("failed on listing customConfigMap: %v", errs.Error())
		}

		//check ccm is used
		for _, ccm := range ccmList.Items {
			configVersion := ccm.Annotations["customConfigMapVersion"]
			configMapName := ccm.Spec.ConfigMapName
			checkConfig := false

			//get all deployment in the namespace
			deploymentList, deperr := clientSet.AppsV1().Deployments(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if deperr != nil {
				klog.Errorf("failed on getting deployment list ", deperr.Error())
			}
			for _, deploy := range deploymentList.Items {
				selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
				if err != nil {
					klog.Infof("Failed get selector from deployment %v", err, time.Now().UTC())
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRSs, err := clientSet.AppsV1().ReplicaSets(ns.Name).List(context.TODO(), options)
				if err != nil {
					klog.Errorf(fmt.Sprintf("Failed on getting deployment list based on label for customConfigMap '%s'", ccm.Name), err, time.Now().UTC())
				}
				//list all revision for this deployment
				for _, rs := range allRSs.Items {
					templateAnnotation := rs.Spec.Template.Annotations
					for key, value := range templateAnnotation {
						if key == "ccm-"+configMapName && value == configVersion {
							checkConfig = true
						}
					}
				}
			}

			//get all statefulset in the namespace
			stsList, stsErr := clientSet.AppsV1().StatefulSets(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if stsErr != nil {
				klog.Errorf("Failed on getting statefulSet ", stsErr.Error())
			}

			for _, sts := range stsList.Items {
				selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
				if err != nil {
					klog.Infof("Failed get selector from deployment %v", err, time.Now().UTC())
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRevisions, err := clientSet.AppsV1().ControllerRevisions(ns.Name).List(context.TODO(), options)
				if err != nil {
					klog.Infof(fmt.Sprintf("Failed on getting sts list based on label  for customConfigMap'%s'", ccm.Name), err, time.Now().UTC())
				}

				for _, rs := range allRevisions.Items {
					stsRevision := appsV1.StatefulSet{}
					er := json.Unmarshal(rs.Data.Raw, &stsRevision)
					if er != nil {
						klog.Infof("failed on unmarshal", er.Error(), time.Now().UTC())
					}
					templateAnnotation := stsRevision.Spec.Template.Annotations
					for key, value := range templateAnnotation {
						if key == "ccm-"+configMapName && value == configVersion {
							checkConfig = true
						}
					}
				}
			}

			//get configmap
			configmap, conferr := clientSet.CoreV1().ConfigMaps(ns.Name).Get(context.TODO(), configMapName, metav1.GetOptions{})
			if conferr != nil {
				klog.Errorf(fmt.Sprintf("Failed on getting configmap '%s'", configMapName), "Error", conferr.Error(), time.Now().UTC())
			}
			if len(configmap.Annotations) != 0 {
				if configmap.Annotations["deployments"] != "" || configmap.Annotations["statefulsets"] != "" {
					if !checkConfig {
						//purge ccm
						err := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(ns.Name).Delete(context.TODO(), ccm.Name, metav1.DeleteOptions{})
						if err != nil {
							klog.Errorf(fmt.Sprintf("Failed on parge customConfigMap '%s'", ccm.Name), "Error", err.Error(), time.Now().UTC())
						} else {
							klog.Infof(fmt.Sprintf("customConfigMap purged successfully '%s'", ccm.Name), time.Now().UTC())
						}
					}
				}
			}
		}

		//list all customSecret
		csList, csErr := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if csErr != nil {
			klog.Errorf("failed on listing customConfigMap: %v", csErr.Error())
		}

		//check cs is used

		for _, cs := range csList.Items {
			secretVersion := cs.Annotations["customSecretVersion"]
			secretName := cs.Spec.SecretName
			checkSecret := false

			//get all deployment in the namespace
			deploymentList, deperr := clientSet.AppsV1().Deployments(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if deperr != nil {
				klog.Errorf("failed on getting deployment list ", deperr.Error())
			}

			for _, deploy := range deploymentList.Items {
				selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
				if err != nil {
					klog.Infof("Failed get selector from deployment %v", err, time.Now().UTC())
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRSs, err := clientSet.AppsV1().ReplicaSets(ns.Name).List(context.TODO(), options)
				if err != nil {
					klog.Errorf(fmt.Sprintf("Failed on getting deployment list based on label for customConfigMap '%s'", cs.Name), err, time.Now().UTC())
				}
				//list all revision for this deployment
				for _, rs := range allRSs.Items {
					templateAnnotation := rs.Spec.Template.Annotations
					for key, value := range templateAnnotation {
						if key == "cs-"+secretName && value == secretVersion {
							checkSecret = true
						}
					}
				}
			}

			//get all statefulset in the namespace
			stsList, stsErr := clientSet.AppsV1().StatefulSets(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if stsErr != nil {
				klog.Errorf("Failed on getting statefulSet ", stsErr.Error())
			}

			for _, sts := range stsList.Items {
				selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
				if err != nil {
					klog.Infof("Failed get selector from deployment %v", err, time.Now().UTC())
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRevisions, err := clientSet.AppsV1().ControllerRevisions(ns.Name).List(context.TODO(), options)
				if err != nil {
					klog.Infof(fmt.Sprintf("Failed on getting sts list based on label  for customConfigMap'%s'", cs.Name), err, time.Now().UTC())
				}

				for _, rs := range allRevisions.Items {
					stsRevision := appsV1.StatefulSet{}
					er := json.Unmarshal(rs.Data.Raw, &stsRevision)
					if er != nil {
						klog.Infof("failed on unmarshal", er.Error(), time.Now().UTC())
					}
					templateAnnotation := stsRevision.Spec.Template.Annotations
					for key, value := range templateAnnotation {
						if key == "cs-"+secretName && value == secretVersion {
							checkSecret = true
						}
					}
				}
			}

			//get secret
			secret, secretErr := clientSet.CoreV1().Secrets(ns.Name).Get(context.TODO(), secretName, metav1.GetOptions{})
			if secretErr != nil {
				klog.Errorf(fmt.Sprintf("Failed on getting secret '%s'", secretName), "Error", secretErr.Error(), time.Now().UTC())
			}
			if len(secret.Annotations) != 0 {
				if secret.Annotations["deployments"] != "" || secret.Annotations["statefulsets"] != "" {
					if !checkSecret {
						//purge ccm
						err := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(ns.Name).Delete(context.TODO(), cs.Name, metav1.DeleteOptions{})
						if err != nil {
							klog.Errorf(fmt.Sprintf("Failed on parge customSecret '%s'", cs.Name), "Error", err.Error(), time.Now().UTC())
						} else {
							klog.Infof(fmt.Sprintf("customSecret purged successfully '%s'", cs.Name), time.Now().UTC())
						}
					}
				}
			}

		}

	}

}
