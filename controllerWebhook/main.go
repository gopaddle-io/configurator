package main

import (
	"crypto/tls"
	"flag"
	"fmt"

	"net/http"

	_ "net/http/pprof"

	"github.com/golang/glog"
)

var ch chan *struct{}

func main() {
	var port = ":8015"
	var parameters WhSvrParameters

	flag.StringVar(&parameters.CertFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.KeyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")

	pair, err := tls.LoadX509KeyPair(parameters.CertFile, parameters.KeyFile)
	if err != nil {
		glog.Errorf("Failed to load key pair: %v", err)
	}

	whsvr := &WebhookServer{
		Server: &http.Server{
			Addr:      fmt.Sprintf(":%v", "8015"),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", GetControllerWebhookStatus)
	mux.HandleFunc("/deploycontroller", whsvr.DeployController)
	mux.HandleFunc("/podcontroller", whsvr.PodConfigController)

	whsvr.Server.Handler = mux

	fmt.Printf("Server listening at %s", port)

	if err := whsvr.Server.ListenAndServeTLS("", ""); err != nil {
		glog.Errorf("Error While establishing connection: %v", err)
		ch <- &struct{}{}
	}

}

// GetControllerWebhookStatus It returns worker status
func GetControllerWebhookStatus(rw http.ResponseWriter, req *http.Request) {
	message := `{"status":"Running"}`
	rw.WriteHeader(200)
	rw.Write([]byte(message))
}
