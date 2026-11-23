{{- define "agni.name" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "agni.labels" -}}
app.kubernetes.io/name: agni
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}

{{- define "agni.selector" -}}
app.kubernetes.io/name: agni
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: server
{{- end -}}

{{- define "agni.dsSelector" -}}
app.kubernetes.io/name: agni
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: datasheets
{{- end -}}

{{- define "agni.claim" -}}
{{- .Values.persistence.existingClaim | default (printf "%s-data" (include "agni.name" .)) -}}
{{- end -}}

{{/* The store and uploads URLs, on the claim unless the values name another. */}}
{{- define "agni.store" -}}
{{- .Values.store | default "file:///var/lib/agni/meta" -}}
{{- end -}}
{{- define "agni.uploads" -}}
{{- .Values.uploads | default "file:///var/lib/agni/files" -}}
{{- end -}}

{{/* The proxies, required, since agni refuses --auth over plain HTTP without them. */}}
{{- define "agni.proxies" -}}
{{- $p := .Values.trustedProxies | default list -}}
{{- if not $p -}}
{{- fail "set trustedProxies to your cluster's pod CIDR, where the ingress controller runs, such as 10.244.0.0/16" -}}
{{- end -}}
{{- join "," $p -}}
{{- end -}}

{{/* agni and agnids trust the CA folder beside the system's roots. */}}
{{- define "agni.caEnv" -}}
- name: SSL_CERT_DIR
  value: /etc/ssl/certs:/etc/agni/ca
{{- end -}}
