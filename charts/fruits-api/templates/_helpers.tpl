{{/* Expand the name of the chart. */}}
{{- define "fruits-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Fully qualified app name. */}}
{{- define "fruits-api.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "fruits-api.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "fruits-api.labels" -}}
helm.sh/chart: {{ include "fruits-api.chart" . }}
{{ include "fruits-api.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: fruits-api
{{- end -}}

{{- define "fruits-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fruits-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "fruits-api.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "fruits-api.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/* Name of the secret holding database credentials. */}}
{{- define "fruits-api.secretName" -}}
{{- if .Values.secret.existingSecret -}}
{{- .Values.secret.existingSecret -}}
{{- else -}}
{{- printf "%s-db" (include "fruits-api.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "fruits-api.configMapName" -}}
{{- printf "%s-config" (include "fruits-api.fullname" .) -}}
{{- end -}}
