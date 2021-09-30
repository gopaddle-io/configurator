---
title:  "Building Configurator with custom Docker repo"
last_modified_at: 2021-08-31T16:00:58-04:00
tags:
  - contribution
  - usage
toc: true
toc_label: "On this page"
author: Gayathri R
---

In this blog, I would like to introduce you to the steps for using custom Docker repository while building Configurator.

![Water photo created by tawatchai07 - www.freepik.com](https://i2.wp.com/blog.gopaddle.io/wp-content/uploads/2021/08/custom-docker-repo-configurator.png?fit=2020%2C1340&ssl=1)

As a pre-requisite, you need to have **golang** and **Docker** CLI installed on your machine. You also need a Kubernetes cluster (version 1.16+). Install kubectl command and connect to the kubernetes cluster.

Pre-requisite
-------------

Fork the project and Clone the project to your local machine

    git clone https://github.com/<your-githubhandle>/configurator.git

Check whether golang is configured

    $ go version
    go version go1.13.3 linux/amd64
    ....
    
    $ echo $GOPATH
    /home/user/codebase
    ....
    
    $ echo $GOHOME
    /home/user/codebase

Check whether Docker CLI works

    $  sudo docker run hello-world
    
    Unable to find image 'hello-world:latest' locally
    latest: Pulling from library/hello-world
    b8dfde127a29: Pull complete 
    Digest: sha256:7d91b69e04a9029b99f3585aaaccae2baa80bcf318f4a5d2165a9898cd2dc0a1
    Status: Downloaded newer image for hello-world:latest
    
    Hello from Docker!
    This message shows that your installation appears to be working correctly.
    
    To generate this message, Docker took the following steps:
     1. The Docker client contacted the Docker daemon.
     2. The Docker daemon pulled the "hello-world" image from the Docker Hub.
        (amd64)
     3. The Docker daemon created a new container from that image which runs the
        executable that produces the output you are currently reading.
     4. The Docker daemon streamed that output to the Docker client, which sent it
        to your terminal.
    
    To try something more ambitious, you can run an Ubuntu container with:
     $ docker run -it ubuntu bash
    
    Share images, automate workflows, and more with a free Docker ID:
     https://hub.docker.com/
    
    For more examples and ideas, visit:
     https://docs.docker.com/get-started/

Verify if Kubernetes connectivity works

    $ kubectl cluster-info
    
    Kubernetes control plane is running at https://35.224.198.88
    GLBCDefaultBackend is running at https://35.224.198.88/api/v1/namespaces/kube-system/services/default-http-backend:http/proxy
    KubeDNS is running at https://35.224.198.88/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
    Metrics-server is running at https://35.224.198.88/api/v1/namespaces/kube-system/services/https:metrics-server:/proxy
    
    To further debug and diagnose cluster problems, use 'kubectl cluster-info dump'.
    

Configuring the Docker repository
---------------------------------

Once the pre-requisites are met, we can start configuring the docker registry in order to build configurator and push it to your private repository.

LogIn to your docker hub account.

    docker login --username <docker-hub-userName> --password <docker-hub-password>

Configure the docker hub repository name and the image tag in the Makefile. Edit the Makefile and change the DOCKER\_IMAGE\_REPO and the DOCKER\_IMAGE\_TAG variables to your docker repository and the tag name with which you prefer to push the newly built docker image.

    $ cd configurator
    $ vi Makefile
    
    ifndef DOCKER_IMAGE_REPO
      DOCKER_IMAGE_REPO=demogp/demo-configurator
    endif
    
    ifndef DOCKER_IMAGE_TAG
      DOCKER_IMAGE_TAG=v1.0
    endif

Now edit the configurator-deployment.yaml file and change the docker repository and the image name from where the configurator controller needs to be pulled from.

    $ cd deploy/
    $ vi configurator-deployment.yaml
    ....
    ....
        spec:
          containers:
          - image: demogp/demo-configurator:v1.0
            imagePullPolicy: Always
            name: configurator
          serviceAccountName: configurator-controller

The repository configurations are complete.

Build and Deploy Configurator
-----------------------------

Move to root of the project directory and execute the make command mentioned below.

    $ cd ../
    $ make clean build push deploy
    ....
    ....
    rm -f configurator
    docker rmi demogp/demo-configurator:v1.0
    Error: No such image: demogp/demo-configurator:v1.0
    Makefile:16: recipe for target 'clean-configurator' failed
    make: [clean-configurator] Error 1 (ignored)
    go mod vendor
    ....
    ....
    go build -o configurator . 
    go: downloading github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
    go: downloading github.com/robfig/cron v1.2.0
    go: downloading github.com/google/go-cmp v0.5.2
    ....
    docker build . -t demogp/demo-configurator:v1.0
    Sending build context to Docker daemon  78.48MB
    Step 1/6 : FROM golang
    latest: Pulling from library/golang
    4c25b3090c26: Pull complete 
    1acf565088aa: Pull complete 
    b95c0dd0dc0d: Pull complete 
    5cf06daf6561: Pull complete 
    4541a887d2a0: Pull complete 
    dcac0686adef: Pull complete 
    9717d2820c6a: Pull complete 
    Digest: sha256:634cda4edda00e59167e944cdef546e2d62da71ef1809387093a377ae3404df0
    Status: Downloaded newer image for golang:latest
     ---> 8735189b1527
    Step 2/6 : MAINTAINER Bluemeric <info@bluemeric.com>
     ---> Running in 1a41655fda14
    Removing intermediate container 1a41655fda14
     ---> ffbd8038390d
    Step 3/6 : RUN mkdir /app/
     ---> Running in d24ca3cc6c44
    Removing intermediate container d24ca3cc6c44
     ---> ae25de38a5fc
    Step 4/6 : WORKDIR /app/
     ---> Running in 86ede46c4736
    Removing intermediate container 86ede46c4736
     ---> 3a6c8e408e7b
    Step 5/6 : Add configurator /app/
     ---> 3c99e28f20d4
    Step 6/6 : CMD ["./configurator"]
     ---> Running in 714c9a7524d0
    Removing intermediate container 714c9a7524d0
     ---> c63e68e4ceb2
    Successfully built c63e68e4ceb2
    Successfully tagged demogp/demo-configurator:v1.0
    docker push demogp/demo-configurator:v1.0
    The push refers to repository [docker.io/demogp/demo-configurator]
    04b1dc245435: Pushed 
    acf8d8aa9ae0: Pushed 
    4538c63ee03d: Mounted from library/golang 
    84140b757a05: Mounted from library/golang 
    9444aade22b2: Mounted from library/golang 
    9889ce9dc2b0: Mounted from library/golang 
    21b17a30443e: Mounted from library/golang 
    05103deb4558: Mounted from library/golang 
    a881cfa23a78: Mounted from library/golang 
    v1.0: digest: sha256:3f21ea83d6a215705bd3bf7d2e9f3ceef55cb6ba05ceb8964848f823b8f2aa16 size: 2215
    kubectl create ns configurator		
    namespace/configurator created
    kubectl apply -f deploy/configurator-serviceaccount.yaml
    serviceaccount/configurator-controller created
    kubectl apply -f deploy/configurator-clusterrole.yaml
    clusterrole.rbac.authorization.k8s.io/configurator created
    kubectl apply -f deploy/configurator-clusterrolebinding.yaml
    clusterrolebinding.rbac.authorization.k8s.io/Configurator created
    kubectl apply -f deploy/crd-customConfigMap.yaml
    Warning: apiextensions.k8s.io/v1beta1 CustomResourceDefinition is deprecated in v1.16+, unavailable in v1.22+; use apiextensions.k8s.io/v1 CustomResourceDefinition
    customresourcedefinition.apiextensions.k8s.io/customconfigmaps.configurator.gopaddle.io created
    kubectl apply -f deploy/crd-customSecret.yaml
    Warning: apiextensions.k8s.io/v1beta1 CustomResourceDefinition is deprecated in v1.16+, unavailable in v1.22+; use apiextensions.k8s.io/v1 CustomResourceDefinition
    customresourcedefinition.apiextensions.k8s.io/customsecrets.configurator.gopaddle.io created
    kubectl apply -f deploy/configurator-deployment.yaml
    deployment.apps/configurator-controller created

Build target ‘build’ builds the configurator controller and creates a new Docker image. ‘push’ pushes the image to the Docker registry and ‘deploy’ deploys the configurator CRDs and the controller to the kubernetes cluster. Once the build is complete, you can see the configurator image in your dockerhub.

![](https://i2.wp.com/blog.gopaddle.io/wp-content/uploads/2021/08/Screenshot-from-2021-08-24-13-47-09.png?resize=1024%2C533&ssl=1)

![](https://i2.wp.com/blog.gopaddle.io/wp-content/uploads/2021/08/Screenshot-from-2021-08-24-13-47-09.png?resize=1024%2C533&ssl=1)

Configurator image on dockerhub

How to validate the deployment ?
--------------------------------

Execute the below kubectl commands to validate if the deploy task has successfully installed the configurator in your kubernets environment.

    $ kubectl get ns
    NAME              STATUS   AGE
    configurator      Active   2m22s
    ....
    
    $ kubectl get crds -n configurator
    NAME                                             CREATED AT
    customconfigmaps.configurator.gopaddle.io        2021-08-24T07:45:45Z
    customsecrets.configurator.gopaddle.io           2021-08-24T07:45:47Z
    ....
    
    $ kubectl get pods -n configurator
    NAME                                       READY   STATUS    RESTARTS   AGE
    configurator-controller-666d6794bb-4lm6c   1/1     Running   0          6m52s
    
    
    $ kubectl get clusterrolebinding | grep Configurator
    Configurator     ClusterRole/configurator 10m

Removing Configurator
---------------------

To clean up the controller artifact and the local docker image, you can run the target clean as below.

    $ make remove clean
    ....
    ....
    kubectl delete -f deploy/configurator-deployment.yaml
    deployment.apps "configurator-controller" deleted
    kubectl delete -f deploy/crd-customConfigMap.yaml
    Warning: apiextensions.k8s.io/v1beta1 CustomResourceDefinition is deprecated in v1.16+, unavailable in v1.22+; use apiextensions.k8s.io/v1 CustomResourceDefinition
    customresourcedefinition.apiextensions.k8s.io "customconfigmaps.configurator.gopaddle.io" deleted
    kubectl delete -f deploy/crd-customSecret.yaml
    Warning: apiextensions.k8s.io/v1beta1 CustomResourceDefinition is deprecated in v1.16+, unavailable in v1.22+; use apiextensions.k8s.io/v1 CustomResourceDefinition
    customresourcedefinition.apiextensions.k8s.io "customsecrets.configurator.gopaddle.io" deleted
    kubectl delete -f deploy/configurator-clusterrolebinding.yaml
    clusterrolebinding.rbac.authorization.k8s.io "Configurator" deleted
    kubectl delete -f deploy/configurator-clusterrole.yaml
    clusterrole.rbac.authorization.k8s.io "configurator" deleted
    kubectl delete -f deploy/configurator-serviceaccount.yaml
    serviceaccount "configurator-controller" deleted
    kubectl delete ns configurator
    namespace "configurator" deleted
    ....
    ....
    rm -f configurator
    docker rmi demogp/demo-configurator:v1.0
    Untagged: demogp/demo-configurator:v1.0
    Deleted: sha256:1f997b671507d230e3e685d434b3e9c678b4cf356ea044448b73ae489794ae24
    Deleted: sha256:dec6aeb58347abf3832e747d4478d6493ed1da39639f5ba10dacb372281f59a2
    Deleted: sha256:0e2e52831fa3e6475b347c40369b9cc3a41e2aaabd232480a244c69a90ab9cf3
    Deleted: sha256:4851458a100d5c34297813abc157b15baf1f25bfbbdf9c1cca8e232b03f31103
    Deleted: sha256:07f715e9deed52886e73de55a223dff83baa071f25264bfad677e8644f377fd7
    Deleted: sha256:1fbf81f2d59e63c727e4b97b7a139de6d1fbf89f6715f8533f4c1e3f018a7f92
    Deleted: sha256:0fed8f83cbe4268f8bd2692972ff3310fb88975a829ae7365662a7f5f8efd525
    

For any queries on how to use or how to contribute to the project, you can reach us on the discord server – [https://discord.gg/dr24Z4BmP8](https://discord.gg/dr24Z4BmP8)