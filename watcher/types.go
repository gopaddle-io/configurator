package watcher

import "github.com/robfig/cron"

type Watcher struct {
	Labels []WatcherLabel `json:"Labels"`
}

type WatcherLabel struct {
	NameSpace string `json:"nameSpace"`
	ConfigMap string `json:"configMap"`
	Secret    string `json:"secret"`
}

type CornJob struct {
	Cron *cron.Cron
}
