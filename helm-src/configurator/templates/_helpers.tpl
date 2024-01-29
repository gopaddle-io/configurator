{{/* vim: set filetype=mustache: */}}

{{/*
Generate certificates for admission-controller webhooks
*/}}
{{- define "admission-controller.gen-certs" -}}
{{- $expiration := (.Values.admissionCA.expiration | int) -}}
{{- if (or (empty .Values.admissionCA.cert) (empty .Values.admissionCA.key)) -}}
{{- $ca :=  genCA "admission-controller-ca" $expiration -}}
{{- template "admission-controller.gen-client-tls" (dict "RootScope" . "CA" $ca) -}}
{{- else -}}
{{- $ca :=  buildCustomCert (.Values.admissionCA.cert | b64enc) (.Values.admissionCA.key | b64enc) -}}
{{- template "admission-controller.gen-client-tls" (dict "RootScope" . "CA" $ca) -}}
{{- end -}}
{{- end -}}

{{/*
Generate client key and cert from CA
*/}}
{{- define "admission-controller.gen-client-tls" -}}
{{- $altNames := list ( include "admission-controller.service.fullname" .RootScope) -}}
{{- $expiration := (.RootScope.Values.admissionCA.expiration | int) -}}
{{- $cert := genSignedCert ( include "admission-controller.fullname" .RootScope) nil $altNames $expiration .CA -}}
{{- $clientCert := default $cert.Cert .RootScope.Values.admissionSecret.cert | b64enc -}}
{{- $clientKey := default $cert.Key .RootScope.Values.admissionSecret.key | b64enc -}}
caCert: {{ .CA.Cert | b64enc }}
clientCert: {{ $clientCert }}
clientKey: {{ $clientKey }}
{{- end -}}


{{- define "admission-controller.fullname" -}}
{{- printf "%s-%s" .Values.admissionController.name .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create secret for admission operator.
*/}}
{{- define "admission-controller.secret.name" -}}
  {{- default "default" .Values.admissionSecret.name -}}
{{- end -}}


{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "configurator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create service name for admission service from chart name or apply override.
*/}}
{{- define "admission-controller.service.name" -}}
{{- if .Values.admissionService.create -}}
{{- default (include "admission-controller.fullname" .) .Values.admissionService.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- default "default" .Values.admissionService.name -}}
{{- end -}}
{{- end -}}


{{/*
Admission operator name
*/}}
{{- define "admission-controller.name" -}}
{{- default .Chart.Name .Values.admissionController.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}


{{/*
Create service fullname for admission service with namespace as domain.
*/}}
{{- define "admission-controller.service.fullname" -}}
{{- default ( printf "%s.%s.svc" (include "admission-controller.service.name" .) .Release.Namespace ) -}}
{{- end -}}