package main

import (
	"os"

	"k8s.io/klog/v2"
)

func main() {

	er := initController()
	if er != nil {
		klog.Errorf("Failed on init cofigurator", er.Error())
		os.Exit(1)
	} else {
		klog.Info("Configurator init process done")
		os.Exit(0)
	}

}
