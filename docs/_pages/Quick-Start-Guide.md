---
permalink: /Quick-Start-Guide/
title: "Quick-Start-Guide"
excerpt: "Building and Deploying Configurator"
toc: true
show_author: false
---

### Supported Versions
  - K8s 1.16+

### Building and Deploying Configurator
Build the source code and the docker image for Configurator. Push the image to registry and deploy configurator in the cluster.
```sh
make clean build push deploy 
```

### Removing Configurator
Remove the configurator deployment from cluster and delete local binary and docker image
```sh
make remove clean 
```

### Deploy Configurator using YAML files
YAML files for deploying the latest version of Configurator is available under the deploy folder.You need to deploy the CRDs, the controller and the service/role binding.
```sh
kubectl apply -f deploy/crd-customConfigMap.yaml
kubectl apply -f deploy/crd-customSecret.yaml
kubectl create ns configurator
kubectl apply -f deploy/configurator-clusterrole.yaml
kubectl apply -f deploy/configurator-clusterrolebinding.yaml
kubectl apply -f deploy/configurator-serviceaccount.yaml
kubectl apply -f deploy/configurator-deployment.yaml
```
Verify if configurator resources are created successfully.
```sh
kubectl get deployment -n configurator
NAME                                 READY   UP-TO-DATE   AVAILABLE   AGE
configurator-controller              1/1     1            1           4h38m
```

### Using Configurator
Once configurator is deployed in the cluster, start creating customConfigMaps. Example customConfigMaps are available under artifacts/examples folder.
Create customConfigMap. This will create a configMap with a postfix.
```sh
kubectl apply -f artifacts/exmaples/example-customConfigMap.yaml
```
List the configMap and make a note of the postfix for the first time.
```sh
kubectl get configMap -n test
NAME               DATA   AGE
testconfig-srseq   1      9s
```
Here "srseq" is the postfix.
Create deployment referencing the newly created configMap and its postfix. Please note the deployment metadata label must contain the config name and the config postfix that was created initially. say <configname>=<postfix>. Under the VolumeMounts section use the complete name of the configMap along with the postfix.
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox-deployment
  labels:
   testconfig: srseq
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
          name: testconfig-srseq
```
From now, you can directly update the customConfigMap and this will create a configMap with a new postfix and will automatically sync up the related deployments with the newly created configMap.
Same functionality applies for secrets as well.

### Listing and Viewing the Custom ConfigMaps and Secrets
```sh
kubectl get ccm -n <namespace>
kubectl get customconfigmap -n <namespace>
kubectl describe ccm -n <namespace>
kubectl describe customconfigmap -n <namespace>
kubectl delete ccm -n <namespace>
kubectl delete customconfigmap -n <namespace>
kubectl get customsecret -n <namespace>
kubectl get ccs -n <namespace>
kubectl describe customsecret -n <namespace>
kubectl describe ccs -n <namespace>
kubectl delete customsecret -n <namespace>
kubectl delete ccs -n <namespace>
```

### Architecture
<img src="https://gopaddle-marketing.s3.ap-southeast-2.amazonaws.com/configurartor-architecture.png">

### Pull Requests
1. Fetch the latest code from master branch and resolve any conflicts before sending a pull request.
2. Make sure you build and test the changes before sending a pull request.
3. Ensure the README is updated with any interface or architecture changes.

## Maintainers
Congurator is maintained by [gopaddle.io](https://gopaddle.io) team.