---
title: "Deploying"
description: "Take a fresh machine or cluster to a running agni against your own GitHub Enterprise Server, signed in through your GitHub App or your identity provider."
---

Say your team keeps its boards and its house rules in a GitHub Enterprise Server inside the
company network. You want one agni server beside it, where each engineer signs in, links a design
from a repo, and checks it under the team's rules. Someone who can't read the repo on GitHub can't
open the design in agni either. This page takes an admin from a fresh machine to that server, in
the order the work happens (agni issue 1804).

[Running the server](../running-the-server/) is the reference for each flag named here. This page
is the walkthrough.

## Before you start

- A machine with Docker, or a Kubernetes cluster with an ingress controller and Helm.
- A DNS name for the server, such as `agni.example.com`, and a certificate for it. With Kubernetes
  the ingress holds the certificate.
- Admin rights on the GitHub Enterprise Server, to register a GitHub App and install it on the org
  that holds the boards.
- The release to run, such as `v0.11.0`. Each release attaches the compose bundle and publishes the
  chart.

## Register a GitHub App

agni reads repos, signs people in and checks access through one GitHub App. Register it on your
GitHub Enterprise Server under the org's Settings, Developer settings, GitHub Apps.

- The callback URL is `https://agni.example.com/signin/github/callback/`. Tick "Enable Device
  Flow", so `agni login` works from a terminal.
- The webhook URL is `https://agni.example.com/hooks/github`, with a secret you make below. Subscribe
  to the "Push" event, so a push to a branch a design follows moves the design.
- Its permissions are read-only access to Contents and Metadata. agni never writes to a repo.
- Install the App on the org, on the repos people link boards and rules from.

Keep the App's client id, such as `Iv1.0123456789abcdef`, and generate a client secret. A person's
token then reads only repos the App is installed on, and only those the person can see.

## Make the secrets

Each secret is a file on the machine, readable by the server's user, uid 10001.

```bash
mkdir -p secrets/connections ca
openssl rand -hex 32 > secrets/connections.key        # seals each person's GitHub token
openssl rand -hex 32 > secrets/github-webhook-secret  # paste the same value into the App
printf '%s' "$APP_CLIENT_SECRET" > secrets/github-client-secret
cp agni.example.com.crt secrets/tls.crt && cp agni.example.com.key secrets/tls.key
sudo chown -R 10001 secrets && chmod 0400 secrets/*.key secrets/github-* secrets/tls.*
```

- Back up `connections.key` with the server's data. The people's tokens in the store are sealed
  with it, and they can't be read without it.
- When your own CA signed the GitHub Enterprise Server's certificate, copy that CA into `ca/`. Leave
  the folder empty otherwise, but make it.

## Start it with the compose bundle

```bash
tar -xzf agni-enterprise-v0.11.0.tar.gz && cd agni-enterprise
cp env.example .env
docker compose up -d
```

Fill in `.env` before `docker compose up`. The lines every server needs:

- `AGNI_PUBLIC_URL=https://agni.example.com`, the URL people open.
- `AGNI_GITHUB_API=https://github.example.com/api/v3`, your server's REST API.
- `AGNI_GITHUB_CLIENT_ID`, the App's client id, and `AGNI_GITHUB_ALLOW=org:acme`, who may sign in.
- `AGNI_TLS_CERT`, `AGNI_TLS_KEY`, `AGNI_CONNECTIONS_KEY_FILE`, `AGNI_GITHUB_CLIENT_SECRET_FILE` and
  `AGNI_GITHUB_WEBHOOK_SECRET_FILE`, the files you made above.
- `AGNI_CA_DIR=./ca`, the CA folder.

`docker compose ps` shows the server healthy once it answers. `AGNI_PORT` is 8443 by default, so
forward 443 to it or set `AGNI_PORT=443`.

- The `data` volume holds the store and the uploads.
- `docker compose --profile datasheets up -d` adds the datasheet workbench on `AGNI_DATASHEETS_PORT`,
  with the same certificate and tokens.

## Or start it with the Helm chart

```bash
kubectl create secret generic agni \
  --from-file=connections-key=secrets/connections.key \
  --from-file=github-client-secret=secrets/github-client-secret \
  --from-file=github-webhook-secret=secrets/github-webhook-secret
kubectl create secret tls agni-tls --cert=secrets/tls.crt --key=secrets/tls.key
kubectl create configmap agni-ca --from-file=ca/
helm install agni oci://ghcr.io/panyam/charts/agni --version 0.11.0 \
  --set publicURL=https://agni.example.com --set ingress.host=agni.example.com \
  --set github.api=https://github.example.com/api/v3 --set github.clientID=Iv1.0123456789abcdef \
  --set 'github.allow={org:acme}' --set 'trustedProxies={10.244.0.0/16}' --set caConfigMap=agni-ca
```

- `trustedProxies` is your cluster's pod range, where the ingress controller runs. The chart refuses
  to render without it.
- The chart makes no Secret, so no secret sits in a values file or in Helm's release history.
- The store and the uploads live on a PersistentVolumeClaim, which `helm uninstall` keeps.
  `--set store=postgres://agni@db/agni` keeps the records in your database instead, with
  `PGPASSWORD` set through `env` from a Secret. `--set uploads=s3://bucket/agni` keeps the files in
  a bucket, with its cache on the claim.
- `connectionSecrets` names a Secret mounted at `/run/agni/connections`, for tokens the server reads
  itself, as the bundle's `AGNI_CONNECTION_SECRETS` folder is.
- `--set datasheets.enabled=true --set datasheets.host=ds.example.com` adds the datasheet workbench
  on its own host.

## Sign in

Open `https://agni.example.com` and sign in with GitHub. The App sends you back signed in, and the
server keeps your GitHub token, sealed, as your own connection. From a terminal:

```bash
agni login --remote https://agni.example.com
```

### Signing in through your identity provider instead

When your company signs people in through Entra ID, Okta or Google Workspace, register a client
there with the redirect URI `https://agni.example.com/signin/oidc/callback/`, and turn on its device
flow if it has one.

- With the bundle, fill in `AGNI_OIDC_ISSUER`, `AGNI_OIDC_CLIENT_ID`, `AGNI_OIDC_ALLOW`, such as
  `domain:example.com`, and `AGNI_OIDC_LABEL`. Put the client's secret in a file under
  `secrets/connections/`, and set `AGNI_OIDC_CLIENT_SECRET=file:/run/agni/connections/<file>`.
- With the chart, add `oidc-client-secret` to the `agni` Secret and set `oidc.issuer`,
  `oidc.clientID`, `oidc.allow` and `oidc.label`.

`/signin` then offers both, and `agni login --with oidc` signs in through the provider. Someone
signed in this way holds no GitHub connection, so they add their own before linking from a private
repo:

```bash
agni login --remote https://agni.example.com --with oidc
agni connections add mine --mine --github-token env:GITHUB_TOKEN --remote https://agni.example.com
```

## Link a design and a pack

The admin gives the server its own connection, so it can read the team's repos, and mounts the
repo that holds a board. A token for scripts comes from the server's store, with `docker compose
exec agni agni token create admin --store file:///var/lib/agni/meta`, or `kubectl exec deploy/agni
-- agni token create admin --store file:///var/lib/agni/meta` on Kubernetes.

```bash
agni connections add hw --seal --github-token env:HW_TOKEN --remote https://agni.example.com
agni mounts add board github://hw/baseboard --ref main --connection hw --remote https://agni.example.com
agni mounts add packs github://hw/agni-packs --ref main --connection hw --remote https://agni.example.com
```

`--seal` reads the token where the CLI runs and seals it in the store, since the server runs in a
container and can't read your machine's variables. `agni connections rotate hw --github-token
env:HW_TOKEN_NEXT --remote https://agni.example.com` replaces it while mounts read through it.

An engineer then links a design from the mount, links the team's pack, and checks the design under
it. Each design names the commit it read.

```bash
agni designs create bb --remote https://agni.example.com
agni designs link bb schematic board/hw/bb.kicad_sch@main --remote https://agni.example.com
agni packs link house packs@main --remote https://agni.example.com
agni check designs/bb --pack house --remote https://agni.example.com
```

No repo to try this with yet? [Try it on a new install](#try-it-on-a-new-install) uses a public
board and agni's sample house rules.

## Try it on a new install

Say you want to see agni review a board before anyone puts a real design in. Antmicro publishes the
baseboard for NVIDIA's Jetson AGX Thor under Apache-2.0, and agni's repo carries house rules for it
in `examples/jetson-project` (agni issue 1805).

Make an empty repo `hw/jetson-agx-thor-baseboard` on your server and install the App on it. Then
copy the board in. Antmicro tags no revisions, so tag the two commits agni's samples name as 1.1.2
and 1.1.3:

```bash
git clone https://github.com/antmicro/jetson-agx-thor-baseboard && cd jetson-agx-thor-baseboard
git tag 1.1.2 3411086e6d6ad2384d0609c53cd6b96323f97c24
git tag 1.1.3 8e087f4f946a925041380699dcde51ae8519f621
git push https://github.example.com/hw/jetson-agx-thor-baseboard.git main --tags
```

Mount it, link the board at 1.1.3, and push the house rules from a checkout of agni's repo:

```bash
agni mounts add jetson github://hw/jetson-agx-thor-baseboard --ref main --connection hw --remote https://agni.example.com
agni designs create jetson --remote https://agni.example.com
agni designs link jetson schematic jetson/jetson-agx-thor-baseboard.kicad_sch@1.1.3 --remote https://agni.example.com
agni designs link jetson board jetson/jetson-agx-thor-baseboard.kicad_pcb@1.1.3 --remote https://agni.example.com
agni packs push examples/jetson-project --id jetson-rules --remote https://agni.example.com
agni review designs/jetson --pack jetson-rules --remote https://agni.example.com
```

The review is "Jetson baseboard design review", with 15 items in five areas. On 1.1.3 about half
of them fail, and each failure names the nets behind it. "Every rail carries a bulk capacitor"
fails on `VCC_IN`, "every I2C bus has pull-ups" lists the camera buses, and the CAN item reads
not-applicable, since the board has no CAN. `agni diff designs/jetson@1.1.2 designs/jetson@1.1.3
--remote https://agni.example.com` then shows what changed between the two revisions.

## Check who can open it

Ask someone who can't read `hw/baseboard` on GitHub to sign in and open `designs/bb`. agni refuses
them as `permission_denied`, naming the repo, whether they ask for the design by name, check it or
fetch one of its files. `agni designs list` leaves it out for them. agni asks GitHub with their own
token each time, and keeps the answer for a minute.

## What leaves the network

The server reads repos, signs people in and checks access through `AGNI_GITHUB_API`, and takes
webhooks from your GitHub Enterprise Server. With OIDC sign-in it also talks to your identity
provider. Pulling the images reaches `ghcr.io`, or the mirror `AGNI_IMAGE` names. The enterprise
exercise below runs the server behind a trap for every other host, and its last step fails if the
server reached one.

## Upgrade and back up

- To upgrade, set `AGNI_VERSION` in `.env` and run `docker compose up -d` again. With the chart, run
  `helm upgrade` with the next `--version` and the values you installed with.
- Back up the `data` volume, or the PersistentVolumeClaim, with `connections.key`. The volume holds
  the store and the uploads. With `store` on postgres or `uploads` in a bucket, back those up as you
  do the rest of your database and buckets.
- An upgrade stops the old server before starting the new one, since one server writes the store.

## Rehearse it first

`make exercise-enterprise` runs every step of this page against a fake GitHub Enterprise Server on
your own machine, from the App's sign-in to the refused read. `make exercise-enterprise-bundle` runs
the same steps against the compose bundle, and `make kind-up exercise-enterprise-helm` against the
chart on a kind cluster. Each prints PASS or FAIL per step, so a rehearsal shows what a real install
will need before you touch your own GitHub.
