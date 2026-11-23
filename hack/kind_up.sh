#!/usr/bin/env bash
# Makes a kind cluster as panyam/ci-actions' kind-ingress makes one in CI (agni issue 1803):
# ingress-nginx on this machine's ports 80 and 443, and the images given loaded into it. A cluster
# that is already up is kept, and the images are loaded again.
#
#   KIND_CLUSTER=agni hack/kind_up.sh ghcr.io/panyam/agni:dev
set -euo pipefail
name=${KIND_CLUSTER:-agni}
# The same controller release kind-ingress installs.
ingress=https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.15.1/deploy/static/provider/kind/deploy.yaml

if ! kind get clusters 2>/dev/null | grep -qx "$name"; then
  kind create cluster --name "$name" --wait 120s --config - <<'YAML'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    labels:
      ingress-ready: "true"
    extraPortMappings:
      - { containerPort: 80, hostPort: 80, protocol: TCP }
      - { containerPort: 443, hostPort: 443, protocol: TCP }
YAML
  kubectl --context "kind-$name" apply -f "$ingress"
  kubectl --context "kind-$name" wait -n ingress-nginx --for=condition=ready pod \
    --selector app.kubernetes.io/component=controller --timeout 180s
  # The admission jobs patch the webhook's CA after the controller is ready, and an Ingress applied
  # before that is refused.
  kubectl --context "kind-$name" wait -n ingress-nginx --for=condition=complete job --all --timeout 120s || true
fi
kubectl config use-context "kind-$name" >/dev/null
[ "$#" -gt 0 ] && kind load docker-image --name "$name" "$@"
echo "kind cluster $name is up, with ingress-nginx on ports 80 and 443"
