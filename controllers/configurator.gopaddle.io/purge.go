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

package configuratorgopaddleio

import (
	clientset "github.com/gopaddle-io/configurator/controllers/configurator.gopaddle.io/purge_helper/clientset"
	cronbuilder "github.com/gopaddle-io/configurator/controllers/configurator.gopaddle.io/purge_helper/cronjob/builder"
	cronconfig "github.com/gopaddle-io/configurator/controllers/configurator.gopaddle.io/purge_helper/cronjob/config"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePurgeCronJob(config *cronconfig.CronJobConfig) error {
	const (
		CRONJOB_STABLE_API = "batch/v1"
		CRONJOB_BETA_API   = "batch/v1beta1"
	)

	//Create new ClientSet
	clientSet, err := clientset.NewClientSet(clientset.WithDefaults())
	if err != nil {
		return errors.Wrap(err, "failed to create Purge ClientSet")
	}

	//Default CronJobConfig
	if config == nil {
		config, err = cronconfig.NewCronJobConfig(cronconfig.WithDefaults(clientSet))
		if err != nil {
			return errors.Wrap(err, "failed to generate CronJobConfig with default options")
		}
	}

	//Create new CronJobBuilder with default functions
	builder := cronbuilder.NewCronJobBuilder(cronbuilder.WithDefaults())

	switch builder.GetCronJobAPIGroupVersion(config) {
	case CRONJOB_STABLE_API:
		//Idempotency check
		_, err := clientSet.BatchV1.
			CronJobs(builder.GetCronJobNamespace(config)).
			Get(clientSet.Ctx, builder.GetCronJobName(config), metav1.GetOptions{})
		if err == nil {
			//CronJob already exists
			return nil
		} else if !k8serrors.IsNotFound(err) {
			//Error is not nil, but it's not because CronJob does not exist
			return errors.Wrap(err, "failed to GET batch/v1 CronJob")
		}
		//CronJob does not exist

		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:            builder.GetCronJobName(config),
				Namespace:       builder.GetCronJobNamespace(config),
				OwnerReferences: builder.GetCronJobOwnerReferences(config),
			},
			Spec: batchv1.CronJobSpec{
				Schedule:          builder.GetCronJobSchedule(config),
				ConcurrencyPolicy: builder.GetCronJobV1ConcurrencyPolicy(config),
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers:       builder.GetCronJobContainers(config),
								RestartPolicy:    builder.GetCronJobRestartPolicy(config),
								Affinity:         builder.GetCronJobAffinity(config),
								NodeSelector:     builder.GetCronJobNodeSelector(config),
								ImagePullSecrets: builder.GetCronJobImagePullSecrets(config),
								Tolerations:      builder.GetCronJobTolerations(config),
							},
						},
					},
				},
				SuccessfulJobsHistoryLimit: builder.GetCronJobSuccessfulJobsHistoryLimit(config),
				FailedJobsHistoryLimit:     builder.GetCronJobFailedJobsHistoryLimit(config),
			},
		}
		_, err = clientSet.BatchV1.CronJobs(config.Namespace).Create(clientSet.Ctx, cronJob, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to create Purge Job CronJob (batch/v1) API object")
		}

		return nil

	case CRONJOB_BETA_API:
		//Idempotency check
		_, err := clientSet.BatchV1beta1.
			CronJobs(builder.GetCronJobNamespace(config)).
			Get(clientSet.Ctx, builder.GetCronJobName(config), metav1.GetOptions{})
		if err == nil {
			//CronJob already exists
			return nil
		} else if !k8serrors.IsNotFound(err) {
			//Error is not nil, but it's not because CronJob does not exist
			return errors.Wrap(err, "failed to GET batch/v1beta1 CronJob")
		}
		//CronJob does not exist

		cronJob := &batchv1beta1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:            builder.GetCronJobName(config),
				Namespace:       builder.GetCronJobNamespace(config),
				OwnerReferences: builder.GetCronJobOwnerReferences(config),
			},
			Spec: batchv1beta1.CronJobSpec{
				Schedule:          builder.GetCronJobSchedule(config),
				ConcurrencyPolicy: builder.GetCronJobV1beta1ConcurrencyPolicy(config),
				JobTemplate: batchv1beta1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers:       builder.GetCronJobContainers(config),
								RestartPolicy:    builder.GetCronJobRestartPolicy(config),
								Affinity:         builder.GetCronJobAffinity(config),
								NodeSelector:     builder.GetCronJobNodeSelector(config),
								ImagePullSecrets: builder.GetCronJobImagePullSecrets(config),
								Tolerations:      builder.GetCronJobTolerations(config),
							},
						},
					},
				},
				SuccessfulJobsHistoryLimit: builder.GetCronJobSuccessfulJobsHistoryLimit(config),
				FailedJobsHistoryLimit:     builder.GetCronJobFailedJobsHistoryLimit(config),
			},
		}
		_, err = clientSet.BatchV1beta1.CronJobs(config.Namespace).Create(clientSet.Ctx, cronJob, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to create Purge Job CronJob (batch/v1beta1) API object")
		}
		return nil

	default:
		return errors.New("invalid CronJob API Group and Version")
	}
}
