{{/*
Check consistency requirements.
*/}}
{{- $expectedName := printf "%s-%s" .Values.csiRclone.storageClassName "secret-annotation" -}}
{{- if .Values.deploy.csi-rclone }}
{{- if not eq .Values.rcloneStorageClass $expectedName }}
{{- fail "ERROR: .Values.rcloneStorageClass does not match " $expectedName ". Please Refer to csi-rclone documentation." }}
{{- end }}
{{- end }}

{{/*
Expand the name of the chart.
*/}}
{{- define "amalthea-sessions.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "amalthea-sessions.fullname" -}}
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
{{- define "amalthea-sessions.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "amalthea-sessions.labels" -}}
helm.sh/chart: {{ include "amalthea-sessions.chart" . }}
{{ include "amalthea-sessions.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "amalthea-sessions.selectorLabels" -}}
app.kubernetes.io/name: {{ include "amalthea-sessions.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "amalthea-sessions.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "amalthea-sessions.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
