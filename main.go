package main

import (
	"flag"
	"fmt"
	"time"

	configController "github.com/gopaddle-io/configurator/controller"
	"github.com/gopaddle-io/configurator/pkg/signals"

	"github.com/gopaddle-io/configurator/watcher"

	clientset "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	informers "github.com/gopaddle-io/configurator/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/klog"
)

func main() {

	var kubeconfig *string
	var cfg *rest.Config
	var err error

	kubeconfig = flag.String("kubeconfig", "", "Path to the kubeconfig file. If not specified, InClusterConfig will be used.")
	flag.Parse()

	if ( *kubeconfig != "" ) {
		klog.Warningf("Using kubeconfig: %s", *kubeconfig)
		cfg, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}

	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
	}

	//trigger previous labels and configmaps
	e := watcher.TriggerWatcher(clientSet)
	if e != nil {
		fmt.Println("failed on triggering watcher for pre-existing labels", e, time.Now().UTC())
	}
	//purge unused configmaps and secrets
	watcher.PurgeJob(clientSet)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	configuratorClientSet, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(clientSet, time.Second*30)
	configInformerFactory := informers.NewSharedInformerFactory(configuratorClientSet, time.Second*30)

	controller := configController.NewController(clientSet, configuratorClientSet,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		configInformerFactory.Configurator().V1alpha1().CustomConfigMaps(),
		kubeInformerFactory.Core().V1().Secrets(),
		configInformerFactory.Configurator().V1alpha1().CustomSecrets())

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	configInformerFactory.Start(stopCh)

	//start contoller
	if err = controller.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}

}
