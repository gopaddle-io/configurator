package controller

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"reflect"

	customSecretv1alpha1 "github.com/gopaddle-io/configurator/pkg/apis/configuratorcontroller/v1alpha1"
	"github.com/gopaddle-io/configurator/watcher"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the customSecret resource
// with the current status of the resource.
func (c *Controller) secretSyncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the customSecret resource with this namespace/name
	customSecret, err := c.customSecretsLister.CustomSecrets(namespace).Get(name)
	if err != nil {
		// The customSecret resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("customSecret '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	secretName := customSecret.Spec.SecretName
	if secretName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: secretName name must be specified", key))
		return nil
	}

	// // Get the configMaps with the name specified in customSecret.spec
	labels := make(map[string]string)
	labels["name"] = secretName
	labels["latest"] = "true"
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: labels,
	})
	secrets, err := c.secretsLister.Secrets(customSecret.Namespace).List(selector)
	// If the resource doesn't exist, we'll create it
	if len(secrets) == 0 {
		_, er := c.kubeclientset.CoreV1().Secrets(customSecret.Namespace).Create(context.TODO(), newSecret(customSecret), metav1.CreateOptions{})
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if er != nil {
			return er
		}
		//start watcher for configmap to listen deployment and statefulset
		secretlabel := watcher.WatcherLabel{}
		secretlabel.NameSpace = customSecret.Namespace
		secretlabel.Secret = customSecret.Spec.SecretName
		go watcher.StartWatcher(secretlabel)
		//store label in file
		arrSecretlabel := watcher.Watcher{}
		arrSecretlabel.Labels = append(arrSecretlabel.Labels, secretlabel)
		err := watcher.StoreLabel(arrSecretlabel)
		if err != nil {
			return err
		}
	}
	// if the secret list not equal to empty than compare the secret with custom secret
	// if there any changes we create a new secret and it will update corresponding deployment and statefulset
	if len(secrets) != 0 {
		var secret *corev1.Secret
		for _, item := range secrets {
			secret = item
		}

		// // If this number of the replicas on the CustomSecret resource is specified, and the
		// // number does not equal the current desired replicas on the configMap, we
		// // should update the configMap resource.
		var data = make(map[string][]byte)
		for k, v := range customSecret.Spec.StringData {
			_, er := b64.StdEncoding.DecodeString(v)
			if er == nil {
				data[k] = []byte(v)
			} else {
				str := b64.StdEncoding.EncodeToString([]byte(v))
				data[k] = []byte(str)
			}
		}
		for k, v := range customSecret.Spec.Data {
			data[k] = v
		}
		//checking secret content is equal or not
		if reflect.DeepEqual(secret.Data, data) == false || secret.Type != customSecret.Spec.Type || reflect.DeepEqual(secret.ObjectMeta.Annotations, customSecret.Spec.SecretAnnotations) == false {
			klog.V(4).Infof("CustomSecret %s  Secret edited", name)
			if reflect.DeepEqual(secret.ObjectMeta.Annotations, customSecret.Spec.SecretAnnotations) == false && reflect.DeepEqual(secret.Data, data) {
				if len(secret.ObjectMeta.Annotations) == 0 && len(customSecret.Spec.SecretAnnotations) == 0 {
					fmt.Println("secret annotation and custom secret annotation is empty")
					return nil
				}
			}
			_, err = c.kubeclientset.CoreV1().Secrets(customSecret.Namespace).Create(context.TODO(), newSecret(customSecret), metav1.CreateOptions{})
			if err == nil {
				//removing configmap latest label from previous configmap
				label := make(map[string]string)
				label["name"] = secretName
				label["customSecretName"] = customSecret.Name
				secret.Labels = label
				secret, err = c.kubeclientset.CoreV1().Secrets(customSecret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
			}
		}

		// // If an error occurs during Update, we'll requeue the item so we can
		// // attempt processing again later. This could have been caused by a
		// // temporary network failure, or any other transient reason.
		if err != nil {
			return err
		}
	}

	c.recorder.Event(customSecret, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// newSecret creates a new Secret for a CustomSecret resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the CustomSecret resource that 'owns' it.
func newSecret(customSecret *customSecretv1alpha1.CustomSecret) *corev1.Secret {
	labels := map[string]string{
		"name":             customSecret.Spec.SecretName,
		"customSecretName": customSecret.Name,
		"latest":           "true",
	}
	name := fmt.Sprintf("%s-%s", customSecret.Spec.SecretName, RandomSequence(5))
	secretName := NameValidation(name)
	//coverting stringdata into data
	var data = make(map[string][]byte)
	for k, v := range customSecret.Spec.StringData {
		_, er := b64.StdEncoding.DecodeString(v)
		if er == nil {
			data[k] = []byte(v)
		} else {
			str := b64.StdEncoding.EncodeToString([]byte(v))
			data[k] = []byte(str)
		}
	}
	for k, v := range customSecret.Spec.Data {
		data[k] = v
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: customSecret.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(customSecret, customSecretv1alpha1.SchemeGroupVersion.WithKind("CustomSecret")),
			},
			Annotations: customSecret.Spec.SecretAnnotations,
			Labels:      labels,
		},
		Data: data,
		Type: customSecret.Spec.Type,
	}
}
