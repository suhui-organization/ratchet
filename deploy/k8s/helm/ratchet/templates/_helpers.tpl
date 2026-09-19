{{- /* 命名与标签助手 */ -}}
{{- define "ratchet.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ratchet.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else if contains .Chart.Name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "ratchet.labels" -}}
app.kubernetes.io/name: {{ include "ratchet.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "ratchet.web.fullname" -}}
{{- printf "%s-web" (include "ratchet.fullname" .) -}}
{{- end -}}

{{- /* 镜像地址拼装：repositoryPrefix 为空时直接用 repository */ -}}
{{- define "ratchet.image" -}}
{{- $prefix := .prefix | default "" -}}
{{- if $prefix -}}
{{- printf "%s/%s:%s" ($prefix | trimSuffix "/") .repository .tag -}}
{{- else -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}
{{- end -}}
