package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomConfigMap
type CustomConfigMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CustomConfigMapSpec `json:"spec"`
}

type CustomConfigMapSpec struct {
	ConfigMapName string            `json:"configMapName"`
	Data          map[string]string `json:"data"`
	BinaryData    map[string][]byte `json:"binaryData"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomConfigMapList is a list of CustomConfigMap resource
type CustomConfigMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomConfigMap `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomSecret
type CustomSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CustomSecretSpec `json:"spec"`
}

type CustomSecretSpec struct {
	SecretName        string            `json:"secretName"`
	StringData        map[string]string `json:"stringData"`
	Data              map[string][]byte `json:"data"`
	Type              corev1.SecretType `json:"type"`
	SecretAnnotations map[string]string `json:"secretAnnotations"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomSecretList is a list of CustomSecret resource
type CustomSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomSecret `json:"items"`
}
