/*
Copyright 2021.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CustomConfigMapSpec defines the desired state of CustomConfigMap
type CustomConfigMapSpec struct {
	ConfigMapName string            `json:"configMapName,omitempty"`
	Data          map[string]string `json:"data,omitempty"`
	BinaryData    map[string][]byte `json:"binaryData,omitempty"`
}

// +genclient
// +genclient:noStatus
//+kubebuilder:object:root=true

// CustomConfigMap is the Schema for the customconfigmaps API
type CustomConfigMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CustomConfigMapSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// CustomConfigMapList contains a list of CustomConfigMap
type CustomConfigMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomConfigMap `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CustomConfigMap{}, &CustomConfigMapList{})
}
