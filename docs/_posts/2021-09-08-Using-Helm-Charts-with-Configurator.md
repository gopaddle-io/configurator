---
title:  "Using Helm Charts with Configurator"
last_modified_at: 2021-09-08T16:00:58-04:00
tags:
  - installation
excerpt: "This blog will focus on the following motives:

*   Installing Configurator using the helm chart.
*   Customizing Configurator helm chart based on requirements.
*   Contributing back to the Configurator project.
"
toc_label: "On this page"
author: Ashvin Kumar
---

This blog will focus on the following motives:

*   Installing Configurator using the helm chart.
*   Customizing Configurator helm chart based on requirements.
*   Contributing back to the Configurator project.

![](https://i1.wp.com/blog.gopaddle.io/wp-content/uploads/2021/09/Screenshot-2021-09-08-at-5.47.44-PM.png?fit=2560%2C1282&ssl=1)

**System Requirements**

Make sure that you have installed helm in your machine and you are connected to a Kubernetes cluster. The chart is qualified for helm version > v3 & Kube version v1.20.8. Follow the documentation from the link to install helm: [https://helm.sh/docs/helm/helm\_version/](https://helm.sh/docs/helm/helm_version/)

    helm versionversion.BuildInfo{Version:"v3.0.2", GitCommit:"19e47ee3283ae98139d98460de796c1be1e3975f", GitTreeState:"clean", GoVersion:"go1.13.5"}

**Installing Configurator using helm chart**

Follow the below steps to directly deploy the Configurator helm package. Make sure that a namespace ‘configurator’ already exists in your cluster. If not, create a namespace with the following command.

    kubectl create namespace configurator

Add the configurator helm repository, by executing the following command :

    helm repo add gopaddle_configurator https://gopaddle-io.github.io/configurator/helm/

Once the command is executed, verify the repository by running the command below. You must see the configurator\_helm repo in the list.

    helm repo list

The output must be similar to this:

    NAME                   URL
    hashicorp              https://helm.releases.hashicorp.com
    gopaddle_configurator  https://gopaddleio.github.io/configurator/helm/

Once you’ve verified the repo, install the helm chart with the following command: _helm install <release\_name> <repo\_name/chart\_name>_

    helm install release1.0.0 gopaddle_configurator/configurator

This installs the Configurator CRDs and the controller in the ‘configurator’ namespace. After you install the helm chart, verify by listing the resources in the corresponding namespace using the following commands.

    kubectl get pods -n configuratorkubectl get crds -n configuratorkubectl get serviceaccounts -n configuratorkubectl get clusterrolebindings -n configurator

The configurator is now ready for use. Here is a reference blog on how to use configurator with the deployments: [https://blog.gopaddle.io/2021/04/01/strange-things-you-never-knew-about-kubernetes-configmaps-on-day-one/](https://blog.gopaddle.io/2021/04/01/strange-things-you-never-knew-about-kubernetes-configmaps-on-day-one/)

**Customizing Configurator helm chart based on requirements**

Sometimes, you may wish to change the Configurator image name, Docker repository, image tag or even include other service charts along with Configurator. Modifying the Configurator helm is pretty straightforward. Make sure you’ve cloned the Configurator GitHub project before proceeding with the next steps.

To clone the project, run the following command:

    git clone https://github.com/gopaddle-io/configurator.git

The helm package needs to be unpacked to modify the helm chart. The zip file will be present at the path configurator/helm in the Configurator project. Choose this option when you want to modify the helm chart configuration. Unzip the file with the following command.

    tar -zxvf <path to .tgz file>

This will extract the contents of the chart in a folder. Once you extract, the helm chart’s file system tree will look like this:

    configurator├── charts├── Chart.yaml├── crds│   ├── crd-customConfigMap.yaml│   └── crd-customSecret.yaml├── templates│   ├── configurator-clusterrolebinding.yaml│   ├── configurator-clusterrole.yaml│   ├── configurator-deployment.yaml│   ├── configurator-serviceaccount.yaml│   └── tests└── values.yaml

The crds directory contains the custom resource definition files — _crd-customConfigMap.yaml_ & _crd-customSecret.yaml_. The templates directory contains the resource’s yaml files, in our case, it contains the roles and role bindings and the configurator service definitions. The charts directory is empty by default. This folder can be used to add your application charts that use Configurator Custom Resource. The _Chart.yaml_ file contains information about the helm, like the chart’s name, description, type etc.

    # Default values for my_chart.# This is a YAML-formatted file.# Declare variables to be passed into your templates.replicaCount: 1replicas: 1namespace: configuratorimage: gopaddle/configurator:latest

You can edit the _values.yaml_ file to your requirements like changing the namespace, replica\_count or the image name, docker repository or the image tag. Make sure that the namespace used in the _values.yaml_ exists in the cluster before you do a helm install. Once the necessary configuration is done, execute the following command to install the charts into your cluster: _helm install <release\_name> <chart\_name>_

    helm install release1.0.0 configurator

This will install the helm chart inside the cluster with the new configurations.

**Contributing back to the configurator project**

To contribute the helm changes back to the Configurator project, you need to package the helm chart with the following command :

    helm package <path to helm chart>

This command will package the charts to a .tgz file. After packaging the helm, you need to give a pull request for code review & merge.

You can take a look at this open-source project @ [https://github.com/gopaddle-io/configurator.git](https://github.com/gopaddle-io/configurator.git).

For any queries on how to use or how to contribute to the project, you can reach us on our discord server — [https://discord.gg/dr24Z4BmP8](https://discord.gg/dr24Z4BmP8)

Image courtesy — [https://www.freepik.com/vectors/technology](https://www.freepik.com/vectors/technology) Technology vector created by stories — [www.freepik.com](http://www.freepik.com/)