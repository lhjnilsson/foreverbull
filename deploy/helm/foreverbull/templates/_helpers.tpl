{{- define "foreverbull.validate" -}}
{{- if and .Values.expose.hostPort (gt (int .Values.replicaCount) 1) }}
{{- fail "replicaCount must be 1 when expose.hostPort is true" }}
{{- end }}
{{- /* The password is expanded raw into POSTGRES_URL/DATABASE_URL
($(POSTGRES_PASSWORD) in foreverbull.postgresURL); Kubernetes does not
URL-encode it, so a set value with reserved characters breaks the DSN.
Generated (randAlphaNum) and local default values are already safe.
The MinIO secret key gets the same rule, so both set credentials obey
one URL-safe contract. */}}
{{- if and .Values.postgres.password (not (regexMatch "^[-A-Za-z0-9._~]+$" (.Values.postgres.password | toString))) }}
{{- fail "postgres.password must be URL-safe (letters, digits and -._~ are safe)" }}
{{- end }}
{{- if and .Values.minio.secretKey (not (regexMatch "^[-A-Za-z0-9._~]+$" (.Values.minio.secretKey | toString))) }}
{{- fail "minio.secretKey must be URL-safe (letters, digits and -._~ are safe)" }}
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

{{- /* $(POSTGRES_PASSWORD) is expanded by Kubernetes from the env var above it. */ -}}
{{- define "foreverbull.postgresURL" -}}
postgres://{{ .Values.postgres.user }}:$(POSTGRES_PASSWORD)@postgres:{{ .Values.postgres.port }}/{{ .Values.postgres.database }}
{{- end }}

{{- define "foreverbull.credentialsSecretName" -}}
foreverbull-credentials
{{- end }}

{{- /*
One credential for Secret foreverbull-credentials. Arguments: list root key
value localDefault generatedDefault secret. generatedDefault "" means random.
A set value wins; for a secret, the public local default on a cluster without
hostPort fails unless credentials.allowDefaults. An empty value is the local default
with hostPort, else the value already in the Secret, else generated.
*/ -}}
{{- define "foreverbull.credential" -}}
{{- $root := index . 0 }}
{{- $key := index . 1 }}
{{- $value := index . 2 | default "" | toString }}
{{- $localDefault := index . 3 }}
{{- $generated := index . 4 }}
{{- $secret := index . 5 }}
{{- $local := $root.Values.expose.hostPort }}
{{- $allowDefaults := and $root.Values.credentials $root.Values.credentials.allowDefaults }}
{{- if $value }}
{{- if and $secret (eq $value $localDefault) (not $local) (not $allowDefaults) }}
{{- fail (printf "%s is the public default %q, and this cluster does not use expose.hostPort, so anyone who knows the chart could log in. Fix: remove %s from your values (or set it to \"\") to generate one, set your own value, or set credentials.allowDefaults=true for a throwaway cluster." $key $value $key) }}
{{- end }}
{{- $value }}
{{- else if $local }}
{{- $localDefault }}
{{- else }}
{{- $secretKey := $key | replace "." "-" }}
{{- $existing := lookup "v1" "Secret" $root.Release.Namespace (include "foreverbull.credentialsSecretName" $root) }}
{{- if and $existing $existing.data (hasKey $existing.data $secretKey) }}
{{- index $existing.data $secretKey | b64dec }}
{{- else if $generated }}
{{- $generated }}
{{- else }}
{{- randAlphaNum 32 }}
{{- end }}
{{- end }}
{{- end }}
