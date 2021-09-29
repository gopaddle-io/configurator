<img src="https://gopaddle-marketing.s3.ap-southeast-2.amazonaws.com/Configurator-sync-service.png" width="50%">

[![StackShare](http://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/gopaddleio/gopaddle)  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)  [![Twitter URL](https://img.shields.io/twitter/url?label=%40configuratork8s&style=social&url=https%3A%2F%2Ftwitter.com%2Fconfiguratork8s)](https://twitter.com/configuratork8s)

[![Discord](https://discordapp.com/api/guilds/864856848279666730/widget.png?style=banner2)](https://discord.gg/dr24Z4BmP8)

# Configurator
Configurator is a version control and a sync service that keeps Kubernetes ConfigMaps and Secrets in sync with the deployment. Configurator uses CRDs to create CustomConfigMaps and CustomSecrets that in turn create ConfigMaps and Secrets with a postfix. As and when a change is detected in the CustomConfigMap or CustomSecret, Configurator automaticatlly generates a new ConfigMap with a new postfix. This acts like a version control for the ConfigMaps.
In order to keep the deployments and statefulsets in sync with the ConfigMap and Secret version, users must start with creating a CustomConfigMap as the first step. This creates a new ConfigMap with a postfix ie., first version. Users then have to reference the ConfigMap along with the postfix in their deployment and satefulset specifications. From them on, users can edit the CustomConfigMap directly. Any change in the CustomConfigMap will be automatically rolled out to all the deployments and statefulsets referencing the initial configMap version. A change in ConfigMap not only creates a new ConfigMap version, but also rolls out a new deployment version. This enables both rolling update and rollback of ConfigMaps in sync with the deployment versions.


# Supported Versions
  - K8s 1.16+

# Contribution and additional resources

## Contribution

1. If you wish to contribute to configurator then check the issue tracker to see if you help out with any of them.
2. If you notice a bug or have a useful feature in mind please raise an issue in the issue tracker so that the contributors can work on it and since it helps the community as a    whole (try labelling the issues so they get reviewed sooner).
3. Create a separate feature branch for new enhancements you wish to add to Configurator.
4. Join our discord community where we discuss open issues and help newcomers to contribute more easily.
### Pull requests

  * We follow the standard ‘Fork and Pull’ workflow.
  * Fetch the latest code from master branch and resolve any conflicts before sending a pull request.
  * Make sure you build and test the changes before sending a pull request.
  * Ensure the README is updated with any interface or architecture changes.

## Additional resources

1. Check out the [ADDITIONAL_RESOURCES](/ADDITIONAL_RESOURCES.md) file for blog posts and videos related to configurator.
2. Raise an issue if you want your blog post to be added to the file.
3. Avoid raising an issue asking for help, instead join the discord community where welcoming contributors will help you navigate around your problem.

# How to use Configurator.

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
