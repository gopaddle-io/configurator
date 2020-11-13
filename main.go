package main

import (
	"gopaddle/configurator/watcher"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		log.Panicln("Kubernetes Client Config location is not provided,\n\t")
	}
	port := ":8050"
	route := mux.NewRouter()
	route.Headers("Content-Type", "application/json", "X-Requested-With", "XMLHttpRequest")

	e := watcher.TriggerWatcher()
	if e != nil {
		fmt.Println("failed on triggering watcher for pre-existing labels", e, time.Now().UTC())
	}
	watcher.PurgeJob()
	route.HandleFunc("/api/watcher", watcher.ConfigRollingUpdate).Methods("POST")
	log.Println("Watcher started listening at '%s'", port)
	http.ListenAndServe(port, route)
}
