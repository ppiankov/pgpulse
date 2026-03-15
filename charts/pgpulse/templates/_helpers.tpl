{{- define "pgpulse.name" -}}
pgpulse
{{- end -}}

{{- define "pgpulse.fullname" -}}
{{- if contains (include "pgpulse.name" .) .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "pgpulse.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "pgpulse.targetFullname" -}}
{{- printf "%s-%s" (include "pgpulse.fullname" .root) .target.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "pgpulse.labels" -}}
app.kubernetes.io/name: {{ include "pgpulse.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "pgpulse.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pgpulse.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
