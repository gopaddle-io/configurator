package main

import (
	"fmt"
	"time"

	configController "github.com/gopaddle-io/configurator/controller"
	"github.com/gopaddle-io/configurator/pkg/signals"

	"github.com/gopaddle-io/configurator/watcher"
	//"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/gopaddle-io/configurator/pkg/client/clientset/versioned"
	informers "github.com/gopaddle-io/configurator/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"k8s.io/klog"
)

func main() {
	// args := os.Args[1:]
	// if len(args) < 1 {
	// 	log.Panicln("Kubernetes Client Config location is not provided,\n\t")
	// }
	// klog.InitFlags(nil)
	// flag.Parse()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	//trigger previous labels and configmaps
	e := watcher.TriggerWatcher()
	if e != nil {
		fmt.Println("failed on triggering watcher for pre-existing labels", e, time.Now().UTC())
	}
	//purge unused configmaps and secrets
	watcher.PurgeJob()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	// cfg, err := clientcmd.BuildConfigFromFlags("", args[0])
	// if err != nil {
	// 	klog.Fatalf("Error building kubeconfig: %s", err.Error(), time.Now().UTC())
	// }

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error(), time.Now().UTC())
	}

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
