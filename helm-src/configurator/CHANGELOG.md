# Changelog for Configurator Helm Chart

## 0.2.0-alpha

* Prefix all resources with `.Release.Name` to avoid conflicts with a future support for multiple installations in the same cluster.

* Use namespace from `.Release.Namespace` instead of a fixed `configurator` value.

* Restructure `values.yaml` to include
  * Resource requests and limits.
  * Image pull policy.
  * Flexible image tag, defaults to `.Chart.AppVersion`.

* Move CRDs to the `templates` directory, allows them to be upgraded with `helm upgrade`.

## 0.1.0

First initial release.
