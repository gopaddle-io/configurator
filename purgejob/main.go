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

package main

import (
	"flag"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gopaddle-io/configurator/purgejob/pkg/purge"
)

var purgeLog = log.Log.WithName("purge-job")

func main() {
	//Bind zap CLI flags
	opts := new(zap.Options)
	opts.BindFlags(flag.CommandLine)

	//Parse all CLI flags
	flag.Parse()

	//Init Logging and flush on exit
	log.SetLogger(zap.New(zap.UseFlagOptions(opts)))

	//Run purge job
	err := purge.PurgeCCMAndCS()
	if err != nil {
		purgeLog.Error(err, "failed Purge job", "target", "CustomConfigMap")
		purgeLog.Error(err, "failed Purge job", "target", "CustomSecret")
		os.Exit(1)
	}
}
