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

package clientset

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/discovery"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	batchv1client "k8s.io/client-go/kubernetes/typed/batch/v1"
	batchv1beta1client "k8s.io/client-go/kubernetes/typed/batch/v1beta1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type ClientSet struct {
	Ctx          context.Context
	AppsV1       *appsv1client.AppsV1Client
	BatchV1      *batchv1client.BatchV1Client
	BatchV1beta1 *batchv1beta1client.BatchV1beta1Client
	CoreV1       *corev1client.CoreV1Client
	Discovery    *discovery.DiscoveryClient
}

type ClientSetOption func(*ClientSet) error

//Create a new ClientSet
func NewClientSet(opts ...ClientSetOption) (*ClientSet, error) {
	c := new(ClientSet)

	var err error
	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build "+
				"ClientSet",
			)
		}
	}

	return c, nil
}

//Generate ClientSet with default values
func WithDefaults() ClientSetOption {
	return func(clientSet *ClientSet) error {
		//Set default context to use with API calls
		clientSet.Ctx = context.TODO()

		//Generate a REST config for kubernetes clients
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return errors.Wrap(err, "failed to generate REST "+
				"config for Purge",
			)
		}

		//Generate apps/v1 client
		clientSet.AppsV1, err = appsv1client.NewForConfig(cfg)
		if err != nil {
			return errors.Wrapf(err, "failed to generate apps/v1"+
				" client for Purge ClientSet",
			)
		}

		//Generage batch/v1 client
		clientSet.BatchV1, err = batchv1client.NewForConfig(cfg)
		if err != nil {
			return errors.Wrap(err, "failed to generate batch/v1"+
				" client for Purge ClientSet",
			)
		}

		//Generage batch/v1beta1 client
		clientSet.BatchV1beta1, err = batchv1beta1client.NewForConfig(cfg)
		if err != nil {
			return errors.Wrap(err, "failed to generate "+
				"batch/v1beta1 client for Purge"+
				" ClientSet",
			)
		}
		//Generage core v1 client
		clientSet.CoreV1, err = corev1client.NewForConfig(cfg)
		if err != nil {
			return errors.Wrapf(err, "failed to generate core v1 "+
				"client for Purge ClientSet",
			)
		}

		//Generate Discovery client
		clientSet.Discovery, err = discovery.NewDiscoveryClientForConfig(cfg)
		if err != nil {
			return errors.Wrapf(err, "failed to generate "+
				"discovery client for PurgeClientSet",
			)
		}

		return nil
	}
}
