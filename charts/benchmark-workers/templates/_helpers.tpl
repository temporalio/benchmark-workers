{{/*
Expand the name of the chart.
*/}}
{{- define "benchmark-workers.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "benchmark-workers.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "benchmark-workers.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "benchmark-workers.labels" -}}
helm.sh/chart: {{ include "benchmark-workers.chart" . }}
{{ include "benchmark-workers.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: benchmark
{{- end }}

{{/*
Selector labels
*/}}
{{- define "benchmark-workers.selectorLabels" -}}
app.kubernetes.io/name: {{ include "benchmark-workers.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: benchmark
{{- end }}

{{/*
Convert single value or array to comma-separated string
*/}}
{{- define "benchmark-workers.toCommaSeparated" -}}
{{- if kindIs "slice" . -}}
{{- join "," . -}}
{{- else -}}
{{- . -}}
{{- end -}}
{{- end }}

{{/*
Get GRPC endpoints as comma-separated string
*/}}
{{- define "benchmark-workers.grpcEndpoints" -}}
{{- include "benchmark-workers.toCommaSeparated" .Values.temporal.grpcEndpoint -}}
{{- end }}

{{/*
Get namespaces as comma-separated string
*/}}
{{- define "benchmark-workers.namespaces" -}}
{{- include "benchmark-workers.toCommaSeparated" .Values.temporal.namespace -}}
{{- end }}

{{/*
Get TLS keys as comma-separated string
Uses array values if provided, otherwise falls back to single value
*/}}
{{- define "benchmark-workers.tlsKeys" -}}
{{- if .Values.temporal.tls.keys -}}
{{- include "benchmark-workers.toCommaSeparated" .Values.temporal.tls.keys -}}
{{- else if .Values.temporal.tls.key -}}
{{- .Values.temporal.tls.key -}}
{{- end -}}
{{- end }}

{{/*
Get TLS certs as comma-separated string
Uses array values if provided, otherwise falls back to single value
*/}}
{{- define "benchmark-workers.tlsCerts" -}}
{{- if .Values.temporal.tls.certs -}}
{{- include "benchmark-workers.toCommaSeparated" .Values.temporal.tls.certs -}}
{{- else if .Values.temporal.tls.cert -}}
{{- .Values.temporal.tls.cert -}}
{{- end -}}
{{- end }}

{{/*
Get TLS CAs as comma-separated string
Uses array values if provided, otherwise falls back to single value
*/}}
{{- define "benchmark-workers.tlsCas" -}}
{{- if .Values.temporal.tls.cas -}}
{{- include "benchmark-workers.toCommaSeparated" .Values.temporal.tls.cas -}}
{{- else if .Values.temporal.tls.ca -}}
{{- .Values.temporal.tls.ca -}}
{{- end -}}
{{- end }}

{{/*
Get existing secrets as comma-separated string
Uses array values if provided, otherwise falls back to single value
*/}}
{{- define "benchmark-workers.existingSecrets" -}}
{{- if .Values.temporal.tls.existingSecrets -}}
{{- include "benchmark-workers.toCommaSeparated" .Values.temporal.tls.existingSecrets -}}
{{- else if .Values.temporal.tls.existingSecret -}}
{{- .Values.temporal.tls.existingSecret -}}
{{- end -}}
{{- end }} 