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

package cronjobbuilder

import (
	cronconfig "github.com/gopaddle-io/configurator/controllers/configurator.gopaddle.io/purge_helper/cronjob/config"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CronJobBuilder struct {
	GetCronJobAffinity                   func(*cronconfig.CronJobConfig) *corev1.Affinity
	GetCronJobAPIGroupVersion            func(*cronconfig.CronJobConfig) string
	GetCronJobV1ConcurrencyPolicy        func(*cronconfig.CronJobConfig) batchv1.ConcurrencyPolicy
	GetCronJobV1beta1ConcurrencyPolicy   func(*cronconfig.CronJobConfig) batchv1beta1.ConcurrencyPolicy
	GetCronJobContainers                 func(*cronconfig.CronJobConfig) []corev1.Container
	GetCronJobImagePullSecrets           func(*cronconfig.CronJobConfig) []corev1.LocalObjectReference
	GetCronJobName                       func(*cronconfig.CronJobConfig) string
	GetCronJobNamespace                  func(*cronconfig.CronJobConfig) string
	GetCronJobNodeSelector               func(*cronconfig.CronJobConfig) map[string]string
	GetCronJobOwnerReferences            func(*cronconfig.CronJobConfig) []metav1.OwnerReference
	GetCronJobSchedule                   func(*cronconfig.CronJobConfig) string
	GetCronJobRestartPolicy              func(*cronconfig.CronJobConfig) corev1.RestartPolicy
	GetCronJobServiceAccountName         func(*cronconfig.CronJobConfig) string
	GetCronJobSuccessfulJobsHistoryLimit func(*cronconfig.CronJobConfig) *int32
	GetCronJobFailedJobsHistoryLimit     func(*cronconfig.CronJobConfig) *int32
	GetCronJobTolerations                func(*cronconfig.CronJobConfig) []corev1.Toleration
}

type CronJobBuilderOption func(*CronJobBuilder)

//Generate a new CronJobBuilder
func NewCronJobBuilder(opts ...CronJobBuilderOption) *CronJobBuilder {
	builder := new(CronJobBuilder)

	for _, opt := range opts {
		opt(builder)
	}

	return builder
}

//Generate CronJobBuilder with default values
func WithDefaults() CronJobBuilderOption {
	return func(b *CronJobBuilder) {
		b.GetCronJobAffinity = func(config *cronconfig.CronJobConfig) *corev1.Affinity {
			if config == nil {
				return nil
			}
			return config.Affinity
		}

		b.GetCronJobAPIGroupVersion = func(config *cronconfig.CronJobConfig) string {
			if config == nil {
				return ""
			}
			return config.APIGroupVersion
		}

		b.GetCronJobV1ConcurrencyPolicy = func(config *cronconfig.CronJobConfig) batchv1.ConcurrencyPolicy {
			if config == nil {
				return ""
			}
			return config.V1ConcurrencyPolicy
		}

		b.GetCronJobV1beta1ConcurrencyPolicy = func(config *cronconfig.CronJobConfig) batchv1beta1.ConcurrencyPolicy {
			if config == nil {
				return ""
			}
			return config.V1beta1ConcurrencyPolicy
		}

		b.GetCronJobContainers = func(config *cronconfig.CronJobConfig) []corev1.Container {
			if config == nil {
				return nil
			}
			return append([]corev1.Container{}, corev1.Container{
				Name:            config.Name,
				ImagePullPolicy: config.ImagePullPolicy,
				Image:           config.Image,
			})
		}

		b.GetCronJobImagePullSecrets = func(config *cronconfig.CronJobConfig) []corev1.LocalObjectReference {
			if config == nil {
				return nil
			}
			return config.ImagePullSecrets
		}

		b.GetCronJobName = func(config *cronconfig.CronJobConfig) string {
			if config == nil {
				return ""
			}
			return config.Name
		}

		b.GetCronJobNamespace = func(config *cronconfig.CronJobConfig) string {
			if config == nil {
				return ""
			}
			return config.Namespace
		}

		b.GetCronJobNodeSelector = func(config *cronconfig.CronJobConfig) map[string]string {
			if config == nil {
				return nil
			}
			return config.NodeSelector
		}

		b.GetCronJobOwnerReferences = func(config *cronconfig.CronJobConfig) []metav1.OwnerReference {
			if config == nil {
				return nil
			}
			return append([]metav1.OwnerReference{}, config.OwnerReference)
		}

		b.GetCronJobRestartPolicy = func(config *cronconfig.CronJobConfig) corev1.RestartPolicy {
			if config == nil {
				return ""
			}
			return config.RestartPolicy
		}

		b.GetCronJobSchedule = func(config *cronconfig.CronJobConfig) string {
			if config == nil {
				return ""
			}
			return config.Schedule
		}

		b.GetCronJobServiceAccountName = func(config *cronconfig.CronJobConfig) string {
			if config == nil {
				return ""
			}
			return config.ServiceAccountName
		}

		b.GetCronJobSuccessfulJobsHistoryLimit = func(config *cronconfig.CronJobConfig) *int32 {
			if config == nil {
				return nil
			}
			return config.SuccessfulJobsHistoryLimit
		}

		b.GetCronJobFailedJobsHistoryLimit = func(config *cronconfig.CronJobConfig) *int32 {
			if config == nil {
				return nil
			}
			return config.FailedJobsHistoryLimit
		}

		b.GetCronJobTolerations = func(config *cronconfig.CronJobConfig) []corev1.Toleration {
			if config == nil {
				return nil
			}
			return config.Tolerations
		}
	}
}
