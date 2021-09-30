---
title:  "Strange things you never knew about ConfigMaps"
last_modified_at: 2021-04-01T16:00:58-04:00
tags:
  - introduction
  - usage
toc: true
toc_label: "On this page"
author: Renugadevi B
---

Deploying on Kubernetes is one complex task, but dealing with surprises during maintenance is another. I can talk from my own experience of maintaining our gopaddle platform deployments on Kubernetes for more than a year now. Even with careful planning and collective knowledge from within the team, there are still hidden challenges in keeping the deployments intact. We get to learn those hidden challenges only by running the production systems on kubernetes for a while. This blog is one such wisdom we learnt. I would like to share our experience with ConfigMaps and what solution we built (open source) to overcome some trouble with ConfigMaps. For the rest of the blog, I am going to be focusing on ConfigMaps, but secrets also have the same set of challenges. Though I have referenced deployments through out the blog, the given scenario and solution exists for kubernetes secrets as well.

![](https://i1.wp.com/blog.gopaddle.io/wp-content/uploads/2021/04/configMaps-strange-things.png?fit=2016%2C1340&ssl=1)

ConfigMaps and Secrets are often overlooked topics when it comes to Cloud Native Deployments. But they can add unforeseen challenges during application maintenance. Let me first introduce you to what ConfigMaps are.

ConfigMaps are Kubernetes resources that are used to store application configurations. It enables build time and run time attribute segregation in a cloud native deployment. Say by using ConfigMaps, you do not have to package application configurations along with your container images. Thus changing application configurations does not require the entire application to be rebuilt. We leverage ConfigMaps to keep our applications 12-factor compatible. In a nutshell, ConfigMaps are:

*   Collection of regular files or key/value pairs
*   Can be used to set Environment variables inside a container (using **ValueFrom: ConfigMapRef** to refer to values defined in ConfigMaps)
*   Can be mounted as directories inside containers (using **VolumeMounts/MountPath** keywords) and all the files within ConfigMaps get mounted inside the container on the mountPath provided.
*   Shared across deployments/replicas
*   Confined to a namespace
*   Created from files, literals, **kustomize configMapGenerator**
*   Replaces all files within the mount path : Since ConfigMaps are mounted inside the container on a given mount path, any existing files and folders within the container will not be available.

Here is an example of how ConfigMaps are defined and referenced inside a deployment specification.

![](https://i1.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configmaps.png?resize=512%2C239&ssl=1)

![](https://i1.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configmaps.png?resize=512%2C239&ssl=1)

Example of defining and using ConfigMaps inside deployment specifications

Hidden challenges of ConfigMaps
-------------------------------

Some of the challenges with ConfigMaps are realized as soon as they are mounted inside the container. But some are unearthered only during maintenance. Following are some challenges we have observed :

1.  **Can’t execute files in ConfigMaps** : Starting from K8s 1.9.6, ConfigMaps get mounted as read-only files by default. Hence you may not be able to execute or run these files. Say, if you are planning on executing these files as container EntryPoint or CMD ARGs, then container may crash during startup as it cannot execute these read-only files. If you have recently upgraded the cluster version, then your deployments may break due this. Please check [this K8s issue](https://github.com/kubernetes/kubernetes/issues/62099) for information on how to configure **ReadOnlyAPIDataVolumes** to mount ConfigMaps as ReadWrite files.
2.  **Deployments & ConfigMaps are loosely coupled,** ie., they follow different lifecycle, but updating the contents of a ConfigMap automatically reflects inside the Pods. More often, applications running inside the containers need a restart to pick the new changes. But, applications are clueless of the changes. These changes are noticed during a scale up/down or a Pod restart event when the application inside the container gets restarted.
3.  **No Versioning/No Rollbacks of ConfigMaps**: ie when deployments are rolled back, it does not roll back the contents of the ConfigMaps.

The last two issues are the resultant of the mutable nature of ConfigMaps.

ConfigMaps are mutable
----------------------

ConfigMaps are mutable ie., they can be edited. Every time a change is made, it is the same ConfigMap that gets updated. There are no revisions. Let me illustrate this with an example.

![](https://i0.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/config-non-immutable.png?resize=512%2C285&ssl=1)

![](https://i0.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/config-non-immutable.png?resize=512%2C285&ssl=1)

ConfigMaps are mutable

We have a ConfigMap, which is referenced in two different deployments. When you change the ConfigMap, the contents of the ConfigMap changes inside the deployments. When the deployments are rolled back, they still point to the current content of the ConfigMap. This can cause a problem when your application is expecting something but it actually sees something else. Deployments do not maintain any state regarding the ConfigMap changes.

Workarounds
-----------

![](https://i0.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configmaps-workarounds.png?resize=768%2C275&ssl=1)

![](https://i0.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configmaps-workarounds.png?resize=768%2C275&ssl=1)

ConfigMaps – Workarounds

1.  **Smart apps:** Applications can be designed in such a way that they constantly poll for changes in the ConfigMaps. This approach still cannot address the roll back issue.
2.  **Induced Rolling Update:** Another common approach is to hash the contents of the ConfigMap in to the deployment. When the ConfigMap changes, the hash changes and that automatically triggers a rolling update on the deployment. But even in this case, rolling back a deployment does not roll back to the previous content of the ConfigMap. [Here](https://blog.questionable.services/article/kubernetes-deployments-configmap-change/) is a reference to how ConfigMap hash can be used.
3.  **Immutable ConfigMaps:** The next option is to use ConfigMap as an immutable content. This feature was introduced in kubernetes 1.19. When this feature is turned on, you cannot update a ConfigMap and thus you can avoid all the associated problems.

Versioning the ConfigMaps
-------------------------

The ideal solution to keep the deployments and the ConfigMaps in sync is to version control the ConfigMaps and reference them in the deployments.

![](https://i2.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configmap-immutable.png?resize=512%2C284&ssl=1)

![](https://i2.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configmap-immutable.png?resize=512%2C284&ssl=1)

Versioning ConfigMaps

*   In the above example, when the contents of the ConfigMap version 1 is updated, it creates a new ConfigMap version 2. When the deployment specification is updated with ConfigMap version 2, it automatically triggers a rolling update and creates a new deployment version. When the deployment is rolled back, the rolled back version will reference ConfigMap v1. Thus ConfigMaps and deployments go hand in hand.

*   To make this work, we need to :

*   **Version ConfigMaps** whenever a change is committed to a ConfigMap
*   **Automatically update ConfigMap versions in Deployment Specifications** where ever it being referenced
*   **Purge unused ConfigMaps periodically** – Since ConfigMaps are shared resources across deployments and since each deployment may have a different revision history limits, we must consider checking all the revisions of all the deployments within the namespace to know if a ConfigMap is being used and purge accordingly.

Introducing Configurator
------------------------

Configurator is an open source solution from gopaddle that makes use of **Custom Resource Definitions (CRDs)** for ConfigMaps/Secrets and an operator to automate the above mentioned steps. ConfigMaps and Secrets are now defined as **CustomConfigMaps** and **CustomSecrets** which are custom resources constantly monitored by the Configurator.

**How it solves the problem**

When a new **CustomConfigMap** or a **CustomSecret** resource is created, it generates a ConfigMap or a Secret with a postfix. This ConfigMap along with the post fix need to be added to the deployments/statefulsets initially.

From then on, if any change to the **CustomConfigMap** or **CustomSecret** is detected, configurator automatically updates all the deployments/statefulsets referencing the specific ConfigMap/Secret with a postfix. Configurator heavily depends on the **configMapName** in the CustomConfigMap and the labels in the deployment/statefulset specifications.

![](https://i0.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configurator-how-it-works.png?resize=1024%2C444&ssl=1)

![](https://i0.wp.com/blog.gopaddle.io/wp-content/uploads/2021/03/configurator-how-it-works.png?resize=1024%2C444&ssl=1)

In the above example you can see that the _example-customConfigMap.yaml_ creates a CustomConfigMap with the **configMapName** as **_testconfig_**. As soon as the CustomConfigMap is created, it automatically creates a ConfigMap **_testconfig-sn8ya_**. We need to manually add this ConfigMapName **_testconfig_** and the postfix **_sn8ya_** in the deployment’s **metadata.labels** as **_testconfig: sn8ya_** and also use the ConfigMapName **_testconfig-sn8ya_** in the volumes section.

From now on, user does not have to manage ConfigMaps directly.

> Any change required in the ConfigMap or Secret needs to be done through the CustomConfigMap or CustomSecret.

When the CustomConfigMap is updated with the new content in the data section, it automatically generates a new ConfigMap **_testconfig-10jov_** and updates the deployment with the new ConfigMap name under the volumes and the **metadata.label** section.

Configurator purges unused ConfigMaps and Secret every 5 mins. It scans the replicaset or controllerRevision of all the deployments and statefulsets in the namespace and checks if the **metadata.label** exists for the ConfigMap. If there are no references, it purges the ConfigMap version.

**How to use it ?**

Download run the YAML files from our repository [here](https://github.com/gopaddle-io/configurator/tree/main/deploy) and install them in your cluster.

    kubectl apply -f deploy/crd-customConfigMap.yaml
    kubectl apply -f deploy/crd-customSecret.yaml
    kubectl create ns configurator
    kubectl apply -f deploy/configurator-clusterrole.yaml
    kubectl apply -f deploy/configurator-clusterrolebinding.yaml
    kubectl apply -f deploy/configurator-serviceaccount.yaml
    kubectl apply -f deploy/configurator-deployment.yaml

Once the configurator is deployed into the cluster, start creating CustomConfigMap or CustomSecret.

 example-customConfigMap.yaml

    apiVersion: "configurator.gopaddle.io/v1alpha1"
    kind: CustomConfigMap
    metadata:
     name: configtest
     namespace: test
    spec:
      configMapName: testconfig
      data:
       application.properties: |
        FOO=Bar

Create CustomConfigMap in cluster

    Kubectl apply -f example-customConfigMap.yaml 

List the ConfigMaps

    kubectl get configMap -n test
    NAME               DATA   AGE
    testconfig-sn8ya   1      7s

Copy the ConfigMap name and add the ConfigMap name in the deployment.yaml file at the volume level and metadata label level. In metadata level split the ConfigMap name and postfix separately and add that in the label.

deployment.yaml

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: busybox-deployment
      labels:
       testconfig: sn8ya
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
              name: testconfig-sn8ya
    

Edit the CustomConfigMap and list the ConfigMap. You can see new ConfigMap name with a postfix.

    Kubectl edit ccm configtest  -n test
    Kubectl list cm -n test
    NAME               DATA   AGE
    testconfig-10jov   1      10s
    testconfig-sn8ya   1      111s

Now check the deployment. You can see that it is updated with new ConfigMap and metadata label.

    kubectl get deployment busybox-deployment -n test -o yaml 
    
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: busybox-deployment
      labels:
      testconfig: 10jov
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
              name: testconfig-10jov

Give configurator a try and share your feedback with us. If you are interested in contributing to the project, you can reach out to us. The project can be cloned from [https://github.com/gopaddle-io/configurator](https://github.com/gopaddle-io/configurator)