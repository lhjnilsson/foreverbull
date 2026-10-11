{{- define "foreverbull.validate" -}}
{{- if and .Values.expose.hostPort (gt (int .Values.replicaCount) 1) }}
{{- fail "replicaCount must be 1 when expose.hostPort is true" }}
{{- end }}
{{- end }}

{{- define "foreverbull.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "foreverbull.labels" -}}
helm.sh/chart: {{ include "foreverbull.chart" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: foreverbull
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
platform: foreverbull
{{- end }}

{{- define "foreverbull.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
{{- end }}

{{- define "foreverbull.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default "foreverbull" .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "foreverbull.image" -}}
{{- $repo := index . 0 }}
{{- $tag := index . 1 }}
{{- $defaultTag := index . 2 }}
{{- printf "%s:%s" $repo ($tag | default $defaultTag) }}
{{- end }}

{{- define "foreverbull.postgresURL" -}}
postgres://{{ .Values.postgres.user }}:{{ .Values.postgres.password }}@postgres:{{ .Values.postgres.port }}/{{ .Values.postgres.database }}
{{- end }}
