# Helm Charts

This directory contains the source code for the helm charts in this project, keeping the chart source code separate from the released packages.

## Conflicts with previous installation methods

Still at early stages of development, a Configurator version may have already been installed using provided Yaml manifests from the `./deploy` directory. That may cause conflicts with the helm chart, preventing a successfull installation on top of it. In order to avoid errors, uninstall the previous version before using the helm chart.

> WARNING: This will also remove CRDs, which also removes all `CustomConfigMaps` and `CustomSecrets` previously created. Make sure to backup before proceeding.

From the project's root directory, run:

```
make remove-configurator
```

## Install using the helm chart

To use the helm chart, add the repository to your local helm installation.

```
helm repo add gopaddle_configurator https://github.com/gopaddle-io/configurator/raw/v0.0.2/helm
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
helm install --create-namespace --namespace configurator configurator gopaddle_configurator/configurator
```

By default, CRDs will be managed and upgraded by the helm chart. If you wish to manage CRDs manually, the value `installCrds=false` can be set during the first installation.

```
# Install CRDs beforehand
make deploy-crds
helm install --create-namespace --namespace configurator configurator gopaddle_configurator/configurator --set installCrds=false
```

## Upgrades

To upgrade to a newer release when it becomes available, run:

```
helm repo up
helm upgrade --namespace configurator configurator gopaddle_configurator/configurator
```

## Uninstall the helm release

A release can be safely removed from the cluster by running:

```
helm uninstall -n configurator configurator
```

Note that CRDs are especially marked, so they will not be removed by Helm. If you wish to remove them as part of a full uninstall, run the `cleanup` target from the `Makefile`.

> **WARNING! Removing CRDs will also automatically remove all `CustomConfigMaps` and `CustomSecrets`, along with all versions of `ConfigMaps` and `Secrets` managed by them. Before proceeding, make sure to backup or adjust impacted dependencies (ie: Pods, Deployments and StatefulSets that might be using them).**

```
make cleanup
```

## Development

Chart development and testing can be done independently from the the indexed repository. Once a new version is ready for release, it needs be packaged and indexed.

To test locally, it is possible to install from the open chart files directly from `./helm-src/configurator`.

```
helm upgrade --install --create-namespace --namespace configurator configurator helm-src/configurator
```

For convenience, the target `helm-install` from the `Makefile` also does exactly that.

```
make helm-install
```

### Packaging a new version

Remember to change `Chart.yaml` to update the values for:

* `version` to reflect the new chart version.
* `appVersion` to reflect the Configurator version.

From the project's root directory, where the file `Makefile` is located, run:

```
make helm
```

A new package will be created in the `./helm` directory and the file `./helm/index.yaml` will be updated. Commit the changes and push.

### Version numering and breaking changes

Developers are encouraged to follow the Semantic Versioning 2.0.0 guidelines to keep version numbers consistent and upgrade behavior predictable.

https://semver.org/

Versions marked as alpha/beta releases are not guaranteed to keep compatibility with previous version and may require a full uninstall/reinstall to work properly.
