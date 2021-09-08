package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"time"

	"math/rand"

	customConfigMapv1alpha1 "github.com/gopaddle-io/configurator/pkg/apis/configuratorcontroller/v1alpha1"
	clientset "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	configscheme "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned/scheme"
	informers "github.com/gopaddle-io/configurator/pkg/client/informers/externalversions/configuratorcontroller/v1alpha1"
	listers "github.com/gopaddle-io/configurator/pkg/client/listers/configuratorcontroller/v1alpha1"
	"github.com/gopaddle-io/configurator/watcher"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformar "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	core "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const controllerAgentName = "configurator"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Configurator is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Configurator fails
	// to sync due to a configmap of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a configmap already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Configurator"
	// MessageResourceSynced is the message used for an Event fired when a Configurator
	// is synced successfully
	MessageResourceSynced = "Configurator synced successfully"
)

// Controller is the controller implementation for configurator resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	configmapsLister      core.ConfigMapLister
	configmapsSynced      cache.InformerSynced
	customConfigMapLister listers.CustomConfigMapLister
	customConfigMapSynced cache.InformerSynced

	secretsLister       core.SecretLister
	secretSynced        cache.InformerSynced
	customSecretsLister listers.CustomSecretLister
	customSecretSynced  cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

type Event struct {
	Kind             string `json:"kind"`
	NameAndNameSpace string `json:"nameAndNameSpace"`
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	configmapInformer coreinformar.ConfigMapInformer,
	customConfigMapInformer informers.CustomConfigMapInformer,
	secretInformer coreinformar.SecretInformer,
	customSecretInformer informers.CustomSecretInformer) *Controller {

	// Create event broadcaster
	// Add configuratorcontroller types to the default Kubernetes Scheme so Events can be
	// logged for configuratorcontroller types.
	utilruntime.Must(configscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:         kubeclientset,
		sampleclientset:       sampleclientset,
		configmapsLister:      configmapInformer.Lister(),
		configmapsSynced:      configmapInformer.Informer().HasSynced,
		customConfigMapLister: customConfigMapInformer.Lister(),
		customConfigMapSynced: customConfigMapInformer.Informer().HasSynced,
		secretsLister:         secretInformer.Lister(),
		secretSynced:          secretInformer.Informer().HasSynced,
		customSecretsLister:   customSecretInformer.Lister(),
		customSecretSynced:    customSecretInformer.Informer().HasSynced,
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Configurators"),
		recorder:              recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when CustomConfigMap resources change
	customConfigMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueConfigurator,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueConfigurator(new)
		},
	})
	// Set up an event handler for when configmap resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a CustomConfigMap resource will enqueue that CustomConfigMap resource for
	// processing. This way, we don't need to implement custom logic for
	// handling configmap resources. More info on this pattern:
	customConfigMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleObject,
		DeleteFunc: controller.handleObject,
	})

	// Set up an event handler for when CustomSecret resources change
	customSecretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueConfigurator,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueConfigurator(new)
		},
	})
	// Set up an event handler for when configmap resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a customSecret resource will enqueue that CustomSecret resource for
	// processing. This way, we don't need to implement custom logic for
	// handling configmap resources. More info on this pattern:
	customSecretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleObject,
		DeleteFunc: controller.handleObject,
	})
	return controller
}

// enqueueConfigurator takes a Configurator resource(customConfigmap|customSecret) and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Configurator.
func (c *Controller) enqueueConfigurator(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	var ownerRef metav1.TypeMeta
	var customConfigMap customConfigMapv1alpha1.CustomConfigMap
	val, _ := json.Marshal(obj)
	json.Unmarshal(val, &ownerRef)
	json.Unmarshal(val, &customConfigMap)
	if ownerRef.Kind == "" {
		annotation := obj.(metav1.Object).GetAnnotations()
		json.Unmarshal([]byte(annotation["kubectl.kubernetes.io/last-applied-configuration"]), &ownerRef)

	}
	if customConfigMap.Spec.ConfigMapName != "" {
		ownerRef.Kind = "CustomConfigMap"
	} else {
		ownerRef.Kind = "CustomSecret"
	}
	event := Event{
		Kind:             ownerRef.Kind,
		NameAndNameSpace: key,
	}
	eventData, _ := json.Marshal(event)
	c.workqueue.Add(string(eventData))
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Configurator resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Configurator resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Configurator, we should not do anything more
		// with it.
		if ownerRef.Kind != "CustomConfigMap" || ownerRef.Kind != "CustomSecret" {
			return
		}
		if ownerRef.Kind != "CustomConfigMap" {
			customConfigMap, err := c.customConfigMapLister.CustomConfigMaps(object.GetNamespace()).Get(ownerRef.Name)
			if err != nil {
				klog.V(4).Infof("ignoring orphaned object '%s' of configurator '%s'", object.GetSelfLink(), ownerRef.Name)
				return
			}

			c.enqueueConfigurator(customConfigMap)
		} else if ownerRef.Kind != "CustomSecret" {
			customSecret, err := c.customSecretsLister.CustomSecrets(object.GetNamespace()).Get(ownerRef.Name)
			if err != nil {
				klog.V(4).Infof("ignoring orphaned object '%s' of configurator '%s'", object.GetSelfLink(), ownerRef.Name)
				return
			}

			c.enqueueConfigurator(customSecret)
		}
		return
	}
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Configurator controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.configmapsSynced, c.customConfigMapSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.secretSynced, c.customSecretSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process configurator resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {

	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}
	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// configurator resource to be synced.
		event := Event{}
		json.Unmarshal([]byte(key), &event)
		if event.Kind == "CustomConfigMap" {
			log.Println("kind: CustomConfigMap Name of the resource ", event.NameAndNameSpace)
			if err := c.syncHandler(event.NameAndNameSpace); err != nil {
				// Put the item back on the workqueue to handle any transient errors.
				c.workqueue.AddRateLimited(key)
				return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
			}
		} else if event.Kind == "CustomSecret" {
			log.Println("kind: CustomSecret Name of the resource ", event.NameAndNameSpace)
			if err := c.secretSyncHandler(event.NameAndNameSpace); err != nil {
				// Put the item back on the workqueue to handle any transient errors.
				c.workqueue.AddRateLimited(key)
				return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
			}
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CustomConfigMap resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CustomConfigMap resource with this namespace/name
	customConfigmap, err := c.customConfigMapLister.CustomConfigMaps(namespace).Get(name)
	if err != nil {
		// The CustomConfigMap resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("CustomConfigMap '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	configmapName := customConfigmap.Spec.ConfigMapName
	if configmapName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: configmap name must be specified", key))
		return nil
	}

	// // Get the configMaps with the name specified in CustomConfigMap.spec
	labels := make(map[string]string)
	labels["name"] = configmapName
	labels["latest"] = "true"
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: labels,
	})
	//options := metav1.ListOptions{LabelSelector: selector.String()}
	configMaps, err := c.configmapsLister.ConfigMaps(customConfigmap.Namespace).List(selector)
	// // If the resource doesn't exist, we'll create it
	if len(configMaps) == 0 {
		cm, er := c.kubeclientset.CoreV1().ConfigMaps(customConfigmap.Namespace).Create(context.TODO(), newConfigmap(customConfigmap), metav1.CreateOptions{})
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if er != nil {
			c.recorder.Eventf(customConfigmap, corev1.EventTypeWarning, "FailedCreateConfigMap", "Error creating ConfigMap: %v", err)
			return er
		}
		c.recorder.Eventf(customConfigmap, corev1.EventTypeNormal, "ConfigMap", "Created ConfigMap: %v", cm.Name)
		//start watcher for configmap to listen deployment and statefulset
		configlabel := watcher.WatcherLabel{}
		configlabel.NameSpace = customConfigmap.Namespace
		configlabel.ConfigMap = customConfigmap.Spec.ConfigMapName
		go watcher.StartWatcher(c.kubeclientset, configlabel, cm.Name, "")
		//store label in file
		arrConfiglabel := watcher.Watcher{}
		arrConfiglabel.Labels = append(arrConfiglabel.Labels, configlabel)
		err = watcher.StoreLabel(arrConfiglabel)
		if err != nil {
			return err
		}
	}

	// if the configmap list not equal to empty than compare the ConfigMap with customConfigMap
	// if there any changes we create a new configmap and it will update corresponding deployment and statefulset
	if len(configMaps) != 0 {
		var configMap *corev1.ConfigMap
		for _, config := range configMaps {
			configMap = config
		}

		// // If this number of the replicas on the CustomConfigMap resource is specified, and the
		// // number does not equal the current desired replicas on the configMap, we
		// // should update the configMap resource.
		if reflect.DeepEqual(configMap.Data, customConfigmap.Spec.Data) == false || reflect.DeepEqual(configMap.BinaryData, customConfigmap.Spec.BinaryData) == false {
			klog.V(4).Infof("CustomConfigMap %s  configmap edited", name)
			cm, err := c.kubeclientset.CoreV1().ConfigMaps(customConfigmap.Namespace).Create(context.TODO(), newConfigmap(customConfigmap), metav1.CreateOptions{})
			if err == nil {
				//removing configmap latest label from previous configmap
				label := make(map[string]string)
				label["name"] = configmapName
				label["customConfigMapName"] = customConfigmap.Name
				configMap.Labels = label
				configMap, err = c.kubeclientset.CoreV1().ConfigMaps(customConfigmap.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				c.recorder.Eventf(customConfigmap, corev1.EventTypeNormal, "ConfigMap", "Created ConfigMap: %v", cm.Name)
			} else {
				c.recorder.Eventf(customConfigmap, corev1.EventTypeWarning, "FailedCreateConfigMap", "Error creating ConfigMap: %v", err)
			}
			//start watcher for configmap to listen deployment and statefulset
			configlabel := watcher.WatcherLabel{}
			configlabel.NameSpace = customConfigmap.Namespace
			configlabel.ConfigMap = customConfigmap.Spec.ConfigMapName
			go watcher.StartWatcher(c.kubeclientset, configlabel, configMap.Name, cm.Name)
			//store label in file
			arrConfiglabel := watcher.Watcher{}
			arrConfiglabel.Labels = append(arrConfiglabel.Labels, configlabel)
			err = watcher.StoreLabel(arrConfiglabel)
			if err != nil {
				return err
			}
		}

		// // If an error occurs during Update, we'll requeue the item so we can
		// // attempt processing again later. This could have been caused by a
		// // temporary network failure, or any other transient reason.
		if err != nil {
			return err
		}
	}

	//c.recorder.Event(customConfigmap, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// newConfigmap creates a new ConfigMap for a CustomConfigMap resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the CustomConfigMap resource that 'owns' it.
func newConfigmap(customConfigmap *customConfigMapv1alpha1.CustomConfigMap) *corev1.ConfigMap {
	labels := map[string]string{
		"name":             customConfigmap.Spec.ConfigMapName,
		"customConfigName": customConfigmap.Name,
		"latest":           "true",
	}
	name := fmt.Sprintf("%s-%s", customConfigmap.Spec.ConfigMapName, RandomSequence(5))
	configName := NameValidation(name)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configName,
			Namespace: customConfigmap.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(customConfigmap, customConfigMapv1alpha1.SchemeGroupVersion.WithKind("CustomConfigMap")),
			},
			Labels: labels,
		},
		Data:       customConfigmap.Spec.Data,
		BinaryData: customConfigmap.Spec.BinaryData,
	}
}

func RandomSequence(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func NameValidation(name string) string {
	reg, err := regexp.Compile("[^a-z0-9-]+")
	if err != nil {
		panic(err)
	}

	return reg.ReplaceAllString(name, "s")
}
