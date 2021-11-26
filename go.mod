module github.com/gopaddle-io/configurator

go 1.15

require (
	github.com/Azure/go-autorest/autorest v0.11.17 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.10 // indirect
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/gophercloud/gophercloud v0.15.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/mitchellh/mapstructure v1.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/robfig/cron v1.2.0
	github.com/stretchr/testify v1.6.1 // indirect
	golang.org/x/mod v0.3.0 // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/sys v0.0.0-20201112073958-5cba982894dd // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	golang.org/x/tools v0.0.0-20200616133436-c1934b75d054 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/api v0.19.0-alpha.1
	k8s.io/apimachinery v0.19.0-alpha.1
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/code-generator v0.19.0-alpha.1
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.4.0 // indirect
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.0.2 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20210115125903-c873f2e8ab25
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20210116005712-af2ce7e24233
	k8s.io/client-go => k8s.io/client-go v0.0.0-20210114130407-537eda74d850
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20210116045519-2a79acd68e5f
)
