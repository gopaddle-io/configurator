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
	"github.com/pkg/errors"
	"k8s.io/client-go/discovery"
)

func getCronJobApiVersion(client *discovery.DiscoveryClient) (string, error) {
	if client == nil {
		return "", errors.New("invalid input: discovery client is nil")
	}

	const (
		CRONJOB_KIND       = "CronJob"
		CRONJOB_STABLE_API = "batch/v1"
		CRONJOB_BETA_API   = "batch/v1beta1"
	)

	//Checking if the batch/v1 API contains the CronJob Kind
	apiList, err := client.ServerResourcesForGroupVersion(CRONJOB_STABLE_API)
	if err != nil {
		return "", errors.Wrapf(err,
			"failed to GET list of resources for the %s API",
			CRONJOB_STABLE_API,
		)
	}
	for _, resource := range apiList.APIResources {
		if resource.Kind == CRONJOB_KIND {
			return CRONJOB_STABLE_API, nil
		}
	}

	//Defaulting to batch/v1beta1 because the project has a prerequisite
	// of at least k8s v1.16 and 1.16 supports batch/v1beta1
	return CRONJOB_BETA_API, nil
}
