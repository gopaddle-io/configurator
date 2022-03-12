/*
Copyright 2022.

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

package purge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	client "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	appsV1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

//it remove unused customConfigMap and customSecret
func PurgeCCMAndCS() error {
	var cfg *rest.Config
	var err error
	cfg, err = rest.InClusterConfig()
	//create clientset
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "error building kubernetes clientset")
	}

	configuratorClientSet, err := client.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "error building example clientset")
	}

	//list all namespace
	nsList, err := clientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed on listing Namespaces")
	}
	for _, ns := range nsList.Items {
		//list all customConfigMap
		ccmList, errs := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if errs != nil {
			return errors.Wrap(errs, "failed on listing CustomConfigMap")
		}

		//check ccm is used
		for _, ccm := range ccmList.Items {
			configVersion := ccm.Annotations["customConfigMapVersion"]
			configMapName := ccm.Spec.ConfigMapName
			checkConfig := false

			//get all deployment in the namespace
			deploymentList, deperr := clientSet.AppsV1().Deployments(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if deperr != nil {
				return errors.Wrap(deperr, "failed on getting deployment list")
			}
			for _, deploy := range deploymentList.Items {
				selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
				if err != nil {
					return errors.Wrap(err, "failed get selector from deployment")
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRSs, err := clientSet.AppsV1().ReplicaSets(ns.Name).List(context.TODO(), options)
				if err != nil {
					return errors.Wrapf(err, "failed on getting deployment list based on label for CustomConfigMap '%s'", ccm.Name)
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
				return errors.Wrap(stsErr, "failed on getting statefulSet")
			}

			for _, sts := range stsList.Items {
				selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
				if err != nil {
					return errors.Wrap(err, "failed get selector from deployment")
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRevisions, err := clientSet.AppsV1().ControllerRevisions(ns.Name).List(context.TODO(), options)
				if err != nil {
					return errors.Wrapf(err, "failed on getting sts list based on label  for CustomConfigMap '%s'", ccm.Name)
				}

				for _, rs := range allRevisions.Items {
					stsRevision := appsV1.StatefulSet{}
					er := json.Unmarshal(rs.Data.Raw, &stsRevision)
					if er != nil {
						return errors.Wrap(er, "failed on unmarshal")
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
				return errors.Wrapf(conferr, "failed on getting configmap '%s'", configMapName)
			}
			if len(configmap.Annotations) != 0 {
				if configmap.Annotations["deployments"] != "" || configmap.Annotations["statefulsets"] != "" {
					if !checkConfig {
						//purge ccm
						err := configuratorClientSet.ConfiguratorV1alpha1().CustomConfigMaps(ns.Name).Delete(context.TODO(), ccm.Name, metav1.DeleteOptions{})
						if err != nil {
							return errors.Wrapf(err, "failed on purge CustomConfigMap '%s'", ccm.Name)
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
			return errors.Wrap(csErr, "failed on listing customConfigMap")
		}

		//check cs is used

		for _, cs := range csList.Items {
			secretVersion := cs.Annotations["customSecretVersion"]
			secretName := cs.Spec.SecretName
			checkSecret := false

			//get all deployment in the namespace
			deploymentList, deperr := clientSet.AppsV1().Deployments(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if deperr != nil {
				return errors.Wrap(deperr, "failed on getting deployment list ")
			}

			for _, deploy := range deploymentList.Items {
				selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
				if err != nil {
					return errors.Wrap(err, "failed get selector from deployment")
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRSs, err := clientSet.AppsV1().ReplicaSets(ns.Name).List(context.TODO(), options)
				if err != nil {
					return errors.Wrapf(err, "failed on getting deployment list based on label for customConfigMap '%s'", cs.Name)
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
				return errors.Wrap(stsErr, "failed on getting statefulSet")
			}

			for _, sts := range stsList.Items {
				selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
				if err != nil {
					return errors.Wrap(err, "failed get selector from deployment")
				}
				options := metav1.ListOptions{LabelSelector: selector.String()}
				allRevisions, err := clientSet.AppsV1().ControllerRevisions(ns.Name).List(context.TODO(), options)
				if err != nil {
					return errors.Wrapf(err, "failed on getting sts list based on label  for customConfigMap'%s'", cs.Name)
				}

				for _, rs := range allRevisions.Items {
					stsRevision := appsV1.StatefulSet{}
					er := json.Unmarshal(rs.Data.Raw, &stsRevision)
					if er != nil {
						return errors.Wrap(er, "failed on unmarshal")
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
				return errors.Wrapf(secretErr, "failed on getting secret '%s'", secretName)
			}
			if len(secret.Annotations) != 0 {
				if secret.Annotations["deployments"] != "" || secret.Annotations["statefulsets"] != "" {
					if !checkSecret {
						//purge ccm
						err := configuratorClientSet.ConfiguratorV1alpha1().CustomSecrets(ns.Name).Delete(context.TODO(), cs.Name, metav1.DeleteOptions{})
						if err != nil {
							return errors.Wrapf(err, "Failed on parge customSecret '%s'", cs.Name)
						} else {
							klog.Infof(fmt.Sprintf("customSecret purged successfully '%s'", cs.Name), time.Now().UTC())
						}
					}
				}
			}
		}
	}

	return nil
}
