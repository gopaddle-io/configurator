# Helm Charts

This directory contains the source code for the helm charts in this project, keeping the chart source code separate from the released packages.

## Conflicts with a previous version installed with Yaml manifests

Still at early stages of development, a Configurator version may have already been installed using provided Yaml manifests from the `./deploy` directory. That may cause conflicts with the helm chart, preventing a successfull installation on top of it. In order to avoid errors, uninstall the previous version before using the helm chart.

> WARNING: This will also remove CRDs, which also removes all `CustomConfigMaps` and `CustomSecrets` previously created. Make sure to backup before proceeding.

From the project's root directory, run:

```
make remove-configurator
```

## Install using the helm chart

To use the helm chart, add the repository to your local helm installation.

```
helm repo add gopaddle_configurator https://gopaddle-io.github.io/configurator/helm
helm repo up
```

Check if you can find the latest version locally.

```
helm search repo gopaddle_configurator
```

The output should be similar to the following:

```
NAME                              	CHART VERSION	APP VERSION	DESCRIPTION                                       
gopaddle_configurator/configurator	0.1.0        	1.16.0     	Helm chart for installing configurator CRDs & C...
```

Install a new release running:

```
kubectl create namespace configurator
helm install --namespace configurator configurator gopaddle_configurator/configurator
```

### Special notes about early releases

At the moment, please take note to follow these steps carefully, creating the namespace `configurator` and including the `--namespace configurator` argument.

### Upgrades

To upgrade to a newer release when it becomes available, simply run:

```
helm repo up
helm upgrade --install --namespace configurator configurator gopaddle_configurator/configurator
```

## Uninstall the helm release

The release can be removed from the cluster running:

```
helm uninstall -n configurator configurator
```

CRDs, however will not be automatically removed. For a full cleanup, CRDs must be removed manually:

> WARNING: Removing CRDs will also remove all `CustomConfigMaps` and `CustomSecrets` already created in the cluster. If you still need them, make sure to backup before proceeding.

```
make cleanup
```

## Development

Chart development and testing can be done independently from the the indexed repository. Once a new version is ready for release, it needs be packaged and indexed.

### Packaging a new version

Remember to change `Chart.yaml` to update the values for:

* `version` to reflect the new chart version.
* `appVersion` to reflect the Configurator version.

From the project's root directory, where the file `Makefile` is located, run:

```
make helm
```

A new package will be created in the `./helm` directory and the file `./helm/index.yaml` will be updated. Commit the changes and push.
