---
title: "Introduction"
permalink: /docs/Introduction/
---

## Configurator
Configurator is a version control and a sync service that keeps Kubernetes ConfigMaps and Secrets in sync with the deployment. Configurator uses CRDs to create CustomConfigMaps and CustomSecrets that in turn create ConfigMaps and Secrets with a postfix. As and when a change is detected in the CustomConfigMap or CustomSecret, Configurator automaticatlly generates a new ConfigMap with a new postfix. This acts like a version control for the ConfigMaps.
In order to keep the deployments and statefulsets in sync with the ConfigMap and Secret version, users must start with creating a CustomConfigMap as the first step. This creates a new ConfigMap with a postfix ie., first version. Users then have to reference the ConfigMap along with the postfix in their deployment and satefulset specifications. From them on, users can edit the CustomConfigMap directly. Any change in the CustomConfigMap will be automatically rolled out to all the deployments and statefulsets referencing the initial configMap version. A change in ConfigMap not only creates a new ConfigMap version, but also rolls out a new deployment version. This enables both rolling update and rollback of ConfigMaps in sync with the deployment versions.