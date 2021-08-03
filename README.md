<img src="https://gopaddle-marketing.s3.ap-southeast-2.amazonaws.com/Configurator-sync-service.png" width="50%">

___


[![StackShare](http://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/gopaddleio/gopaddle)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
![Discord Banner 2](https://discordapp.com/api/guilds/864856848279666730/widget.png?style=banner2)

# Configurator

Configurator is a sync service that keeps Kubernetes deployments in sync with the ConfigMap updates. 

[<img src="https://gopaddle-marketing.s3-ap-southeast-2.amazonaws.com/addo-configurator.png" width="80%">](https://youtu.be/sTX8RASHMXQ?start=2:30:30)

# Supported Versions

  - K8s 1.16+

### Building and Deploying Configurator
Build the source code and the docker image. Push the image to registry and deploy configurator in cluster
```sh
make clean build deploy
```
### Removing Configurator
Remove the configurator deployment from cluster and delete local binary and docker image 
```sh
make remove clean 
```

### How to use ?
Once configurator is deployed in cluster, start creating customConfigMaps. Example customConfigMaps are available under artifacts/examples folder.

Create customConfigMap. This will create a configMap with a postfix
```sh
kubectl apply -f example-customConfigMap.yaml
```
List the configMap and make a note of the postfix for the first time.

```sh
kubectl get configMap -n test
NAME               DATA   AGE
testconfig-srseq   1      9s
```
Here  "srseq" is the postfix.

Create deployment with the newly created configMap along with the postfix. Please note the metadata label must contain the config name and the config postfix that was created initially. say <configname>=<postfix>. Under the VolumeMounts section use the complete name of the configMap along with the postfix.

```sh
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
Whenever the customConfigMap is updated, Configurator will create a configMap with a new postfix and will automatically sync up the related deployments with the newly created configMap.

Same functionality applies for secrets as well.

