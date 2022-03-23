<img src="https://gopaddle-marketing.s3.ap-southeast-2.amazonaws.com/Configurator-sync-service.png" width="50%">

[![StackShare](http://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/gopaddleio/gopaddle)  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)  [![Twitter URL](https://img.shields.io/twitter/url?label=%40configuratork8s&style=social&url=https%3A%2F%2Ftwitter.com%2Fconfiguratork8s)](https://twitter.com/configuratork8s)

[![Discord](https://discordapp.com/api/guilds/864856848279666730/widget.png?style=banner2)](https://discord.gg/dr24Z4BmP8)

# Configurator
Configurator is a version control and a sync service that keeps Kubernetes ConfigMaps and Secrets in sync with the deployments. When a ConfigMap content is changed, Configurator creates a custom resource of type CustomConfigMap (CCM) with a postfix. CCM with a postfix acts like ConfigMap revision. Configurator then copies the modified contents of the ConfigMap in to the CCM resource and triggers a rolling update on deployments using the ConfigMap.  Configurator keeps the ConfigMap contents in sync with the deployment revisions with the help of annotations and works well for both rolling updates and rollbacks. Configurator supports GitOps workflows as well.

# Supported Versions
  - K8s 1.16+

# Contributing
Check the [CONTRIBUTING.md](/CONTRIBUTING.md) file to start contributing to the project

Check out the [Configurator website](https://gopaddle-io.github.io/configurator/) for quick and easy navigation of all documentaion and additional resources. 

Join the community at our [discord server]((https://discord.gg/dr24Z4BmP8))

# How to install Configurator
Configurator can be installed using Helm chart.

### Pre-requisite
* [Install Helm](https://helm.sh/docs/intro/install/)
* [Install kubectl in your local environment](https://kubernetes.io/docs/tasks/tools/)
* Add the contents of the Kubernetes configuration file to your local  ~/.kube/config file
* Check if you could access your kubernetes cluster using kubectl command
```sh
$ kubectl version
```

### Add configurator helm repository
Choose Configurator helm repostry based on the Configuration version. To use Configurator version 0.0.2, add the repo below:
```sh
$ helm repo add gopaddle_configurator https://github.com/gopaddle-io/configurator/raw/v0.0.2/helm
```

### Install configurator to cluster
To install Configurator in the cluster.
```sh
$ helm install configurator gopaddle_configurator/configurator --version 0.4.0-alpha
```

### Removing Configurator
To remove Configurator from the cluster.
```sh
$ helm delete configurator gopaddle_configurator/configurator
```

### License 

[Apache License Version 2.0](/LICENSE.md)

### Pull Requests
1. Fetch the latest code from master branch and resolve any conflicts before sending a pull request.
2. Make sure you build and test the changes before sending a pull request.
3. Ensure the README is updated with any interface or architecture changes.

## Maintainers
Configurator is maintained by [gopaddle.io](https://gopaddle.io) team.
