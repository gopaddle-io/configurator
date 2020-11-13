# Configurator

Configurator is a sync service that keeps Kubernetes deployments in sync with the ConfigMap updates. 


When used with kustomize, 
  - Resource postfix as way to version control ConfigMaps
  - Automatically triggers a rolling update on deployments referring to a configMap version
  - Periodically purges unused ConfigMap versions

[![Watch the video](https://gopaddle-marketing.s3-ap-southeast-2.amazonaws.com/configurator.png)](https://youtu.be/cLi952bntvw)

# Supported Versions

  - K8s 1.16

### Dependencies
Install kustomize in the Kuberentes client environment from where configMap creation needs to be triggered.
  - [kustomize](https://kustomize.io/)

### Building Configurator
Build the source code
```sh
dep ensure
go build main.go
```

### How to use ?
Once configurator is compiled, start the service.

```sh
./main ~/.kube/config
curl -X POST -d '{"labels":[{"nameSpace":"default","configMap":"my-java"}]}' http://localhost:8050/api/watcher
```

Use kustomize to create or update configMaps. Say kustmization.yaml file must look like this. Note that the labels under option section must contain name=<configmapname>

```sh
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# These labels are added to all configmaps and secrets.
configMapGenerator:
- name: my-java
  behavior: create
  literals:
  - JAVA_TOOL_OPTIONS=-agentlib:hprof
  - JAVA_TEST=javatest
  - JAVA_HOME=/home/vino/addo
  options:
    disableNameSuffixHash: false
    labels:
      name: my-java
```
Build the configMap using the kustomization.yaml file.

```sh
kustomize build . | kubectl apply -f -
```

Create deployment with the newly created configMap along with the postfix. Please note the metadata label must contain the config name and the config postfix that was created initially. say <configname>=<postfix>. Under the VolumeMounts section use the complete name of the configMap along with the postfix.

```sh
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox-deployment
  labels:
    my-java: d6f8m44fmg
    app: busybox
spec:
  replicas: 1
  revisionHistoryLimit: 1
  strategy: 
    type: RollingUpdate
  selector:
    matchLabels:
      app: busybox
  template:
    metadata:
      labels:
        app: busybox
    spec:
      containers:
      - name: busybox
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ['sh', '-c', 'echo Container 1 is Running ; sleep 3600']
        volumeMounts:
        - mountPath: /test
          name: test-config
      volumes:
      - name: test-config
        configMap:
          name: my-java-d6f8m44fmg
```
Whenever a new configMap is created through kustomize, Configurator will automatically sync up the related deployments automatically.

### Todos
 - Sync Secretes with Deployments
 - Handle Stateful Sets
 - Move Configurator to Operator Framework

License
----

Apache License 2.0

