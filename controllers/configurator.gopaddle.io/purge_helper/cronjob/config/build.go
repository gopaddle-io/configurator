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

package cronjobconfig

import (
	"io/ioutil"
	"os"

	clientset "github.com/gopaddle-io/configurator/controllers/configurator.gopaddle.io/purge_helper/clientset"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CronJobConfig struct {
	Affinity                   *corev1.Affinity
	APIGroupVersion            string
	V1ConcurrencyPolicy        batchv1.ConcurrencyPolicy
	V1beta1ConcurrencyPolicy   batchv1beta1.ConcurrencyPolicy
	ContainerName              string
	Image                      string
	ImagePullPolicy            corev1.PullPolicy
	ImagePullSecrets           []corev1.LocalObjectReference
	Name                       string
	Namespace                  string
	NodeSelector               map[string]string
	OwnerReference             metav1.OwnerReference
	RestartPolicy              corev1.RestartPolicy
	Schedule                   string
	ServiceAccountName         string
	SuccessfulJobsHistoryLimit *int32
	FailedJobsHistoryLimit     *int32
	Tolerations                []corev1.Toleration
}

type CronJobConfigOption func(*CronJobConfig) error

//Generate a new CronJobConfig
func NewCronJobConfig(opts ...CronJobConfigOption) (*CronJobConfig, error) {
	config := new(CronJobConfig)

	var err error
	for _, opt := range opts {
		err = opt(config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build "+
				"CronJobConfig",
			)
		}
	}

	return config, nil
}

//Generate CronJobConfig with default values
func WithDefaults(clientSet *clientset.ClientSet) CronJobConfigOption {
	return func(config *CronJobConfig) error {
		if clientSet == nil {
			return errors.New("failed to set default values to " +
				"CronJobConfig: invalid ClientSet")
		}

		const (
			POD_NAME_FILE_PATH          = "/etc/podinfo/name"
			POD_NAMESPACE_FILE_PATH     = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
			CONFIGURATOR_CONTAINER_NAME = "configurator"
			CRONJOB_CONTAINER_NAME      = CONFIGURATOR_CONTAINER_NAME + "-" + "purge"
			PURGE_JOB_IMAGE_ENV_KEY     = "PURGE_JOB_IMAGE"
			//Scheduled to run every 15th minute of every hour
			CRONJOB_SCHEDULE           = "*/15 * * * *"
			V1_CONCURRENCY_POLICY      = batchv1.ReplaceConcurrent
			V1BETA1_CONCURRENCY_POLICY = batchv1beta1.ReplaceConcurrent
			RESTART_POLICY             = corev1.RestartPolicyOnFailure
		)

		var history_limit int32 = 1

		//Set Parameters
		config.ContainerName = CRONJOB_CONTAINER_NAME
		config.Schedule = CRONJOB_SCHEDULE
		config.V1ConcurrencyPolicy = V1_CONCURRENCY_POLICY
		config.V1beta1ConcurrencyPolicy = V1BETA1_CONCURRENCY_POLICY
		config.SuccessfulJobsHistoryLimit = &history_limit
		config.FailedJobsHistoryLimit = &history_limit
		config.RestartPolicy = RESTART_POLICY

		//Get Pod Name
		podName, err := ioutil.ReadFile(POD_NAME_FILE_PATH)
		if err != nil {
			return errors.Wrapf(err, "failed to get Pod Name: "+
				"failed to read file %s",
				POD_NAME_FILE_PATH,
			)
		}

		//Get Pod Namespace
		namespaceByteSlice, err := ioutil.ReadFile(POD_NAMESPACE_FILE_PATH)
		if err != nil {
			return errors.Wrapf(err, "failed to get Pod Namespace: "+
				"failed to read file %s",
				POD_NAMESPACE_FILE_PATH,
			)
		}
		//Set CronJobConfig Namespace (same as this Pod)
		config.Namespace = string(namespaceByteSlice)

		//GET the controller Pod (this Pod) from k8s API server
		podObj, err := clientSet.CoreV1.Pods(config.Namespace).
			Get(clientSet.Ctx, string(podName), metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to GET Pod %s in %s "+
				"namespace",
				podName,
				config.Namespace,
			)
		}

		//Set parameters
		config.Affinity = podObj.Spec.Affinity.DeepCopy()
		config.NodeSelector = podObj.Spec.NodeSelector
		config.ImagePullSecrets = podObj.Spec.ImagePullSecrets
		config.ServiceAccountName = podObj.Spec.ServiceAccountName
		config.Tolerations = podObj.Spec.Tolerations

		for index, container := range podObj.Spec.Containers {
			if container.Name == CONFIGURATOR_CONTAINER_NAME {
				//Set CronJobConfig ImagePullPolicy
				config.ImagePullPolicy = podObj.Spec.Containers[index].ImagePullPolicy
				break
			}
		}

		//Set CronJobConfig API Group Version (batch/v1 or batch/v1beta1)
		config.APIGroupVersion, err = getCronJobApiVersion(clientSet.Discovery)
		if err != nil {
			return errors.Wrap(err, "failed to CronJob API Group Version")
		}

		//Set CronJobConfig image
		ok := false
		config.Image, ok = os.LookupEnv(PURGE_JOB_IMAGE_ENV_KEY)
		if !ok {
			return errors.New("failed to set CronJobConfig image " +
				"from env " + PURGE_JOB_IMAGE_ENV_KEY)
		}

		//Get the owning ReplicaSet of this Pod from the Pod's OwnerRef
		if len(podObj.OwnerReferences) == 0 {
			return errors.Errorf("failed to get %s Pod's "+
				"OwnerReference in %s namespace, Pod"+
				" has no OwnerReferences",
				podObj.Name,
				podObj.Namespace,
			)
		} else if podObj.OwnerReferences[0].Kind != "ReplicaSet" {
			return errors.Errorf("expected %s Pod in %s namespace"+
				" to have ReplicaSet owner",
				podObj.Name,
				podObj.Namespace,
			)
		}

		//GET the owning ReplicaSet
		rsObj, err := clientSet.AppsV1.
			ReplicaSets(podObj.Namespace).
			Get(clientSet.Ctx,
				podObj.OwnerReferences[0].Name,
				metav1.GetOptions{},
			)
		if err != nil {
			return errors.Wrapf(err, "failed to GET ReplicaSet %s"+
				" in %s namespace",
				podObj.OwnerReferences[0].Name,
				podObj.Namespace,
			)
		}

		if len(rsObj.OwnerReferences) == 0 {
			return errors.Errorf("failed to get %s ReplicaSet's "+
				"OwnerReference in %s namespace, ReplicaSet "+
				"has no OwnerReferences",
				rsObj.Name,
				rsObj.Namespace,
			)
		}
		//We're not checking if it is owned by a Deployment, because the
		// controller may be deployed as a StatefulSet also

		//Set OwnerReference and Name
		config.OwnerReference = *rsObj.OwnerReferences[0].DeepCopy()
		config.Name = rsObj.OwnerReferences[0].Name + "-" + "purge"

		return nil
	}
}
