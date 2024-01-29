module github.com/gopaddle-io/configurator/controllerInit

go 1.16

require (
	github.com/gopaddle-io/configurator v0.0.2-a
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog/v2 v2.4.0
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20210115125903-c873f2e8ab25
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20210116005712-af2ce7e24233
	k8s.io/client-go => k8s.io/client-go v0.0.0-20210114130407-537eda74d850
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20210116045519-2a79acd68e5f
)
