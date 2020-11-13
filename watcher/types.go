package watcher

import "github.com/robfig/cron"

type Config struct {
	Labels []ConfigLabel `json:"Labels"`
}

type ConfigLabel struct {
	NameSpace string `json:"nameSpace"`
	ConfigMap string `json:"configMap"`
}

type CornJob struct {
	Cron *cron.Cron
}
