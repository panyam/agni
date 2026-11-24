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

{{/* The serve.yaml agni serve --config reads (agni issue 1853). It holds the chart's settings, then
the values' serve section for the rest, then the bootstrap. A key the chart sets wins over the same
key in serve, since the pod's ports, mounts and Secret are built around it. */}}
{{- define "agni.serveConfig" -}}
{{- $publicURL := required "set publicURL, the URL people open, such as https://agni.example.com" .Values.publicURL -}}
{{- $clientID := required "set github.clientID, the GitHub App's client id" .Values.github.clientID -}}
{{- $allow := .Values.github.allow | default list -}}
{{- if not $allow }}{{ fail "set github.allow, such as [org:acme]" }}{{ end -}}
{{- $github := dict "client_id" $clientID "client_secret" "file:/run/secrets/agni/github-client-secret" "allow" $allow "webhook_secret" "file:/run/secrets/agni/github-webhook-secret" -}}
{{- with .Values.github.api }}{{ $_ := set $github "api" . }}{{ end -}}
{{- $cfg := dict "addr" ":8080" "store" (include "agni.store" .) "uploads" (include "agni.uploads" .) "auth" "tokens" "trusted_proxy" (splitList "," (include "agni.proxies" .)) "connections_key" "file:/run/secrets/agni/connections-key" "public_url" $publicURL "github" $github -}}
{{- if regexMatch "^(s3|gs)://" (include "agni.uploads" .) }}{{ $_ := set $cfg "uploads_cache" "/var/lib/agni/uploads-cache" }}{{ end -}}
{{- $oidc := .Values.oidc | default dict -}}
{{- if $oidc.issuer -}}
{{- if not $oidc.clientID }}{{ fail "set oidc.clientID, the client registered with oidc.issuer" }}{{ end -}}
{{- if not $oidc.allow }}{{ fail "set oidc.allow, such as [domain:example.com]" }}{{ end -}}
{{- $o := dict "issuer" $oidc.issuer "client_id" $oidc.clientID "allow" $oidc.allow -}}
{{- if $oidc.clientSecret }}{{ $_ := set $o "client_secret" "file:/run/secrets/agni/oidc-client-secret" }}{{ end -}}
{{- with $oidc.label }}{{ $_ := set $o "label" . }}{{ end -}}
{{- $_ := set $cfg "oidc" $o -}}
{{- end -}}
{{- if .Values.localAccounts }}{{ $_ := set $cfg "local_accounts" true }}{{ end -}}
{{- $cfg = merge $cfg (deepCopy (.Values.serve | default dict)) -}}
{{- with .Values.bootstrap }}{{ $_ := set $cfg "bootstrap" . }}{{ end -}}
{{- toYaml $cfg -}}
{{- end -}}
