Install sops:
https://github.com/mozilla/sops
Create GCP KMS key to encrypt secrets:
https://github.com/mozilla/sops#encrypting-using-gcp-kms
Encrypt secrets
https://github.com/mozilla/sops#encrypting-using-gcp-kms

manually create GKE cluster: https://cloud.google.com/kubernetes-engine/docs/quickstart
manually create GCP postgres instance and 2 tables for dev

install helm: https://helm.sh/docs/install/
deploy till with k8s perms: https://gist.github.com/mgoodness/bd887830cd5d483446cc4cd3cb7db09d

install nginx ingress controller: https://github.com/helm/charts/tree/master/stable/nginx-ingress
helm install stable/nginx-ingress --name nginx-ingress-ctl --namespace nginx -f infra/development/nginx.values.yaml

manually create DNS record for dev SA to point to nginx ingress loadbalancer

create secrets:
<!-- todo: decrypt with sops then create -->
kl create -f deploy/development/sa.env.secret.yaml --namespace dev
kl create -f deploy/development/sa.identity.secret.yaml --namespace dev

helm install deploy/charts/satellite --name satellite-dev --namespace dev -f deploy/development/sa.values.yaml
to delete chart: helm del --purge satellite-dev


Skaffold:
skaffold docs: https://skaffold.dev/docs/how-tos/deployers/#deploying-with-kustomize
$ skaffold config set default-repo gcr.io/storj-jessica/sa-k8s-hard-way
$ skaffold config set --kube-context gke_storj-jessica_us-central1-a_k8s-the-hard-way
$ skaffold run

problems w/skaffold:
1. dockerfile must be in root of directory called Dockerfile, can't currently figure out how to tell it to look for a different place
2. no ability to provide custom scripts for deploys, therefor can't decrypt secrets

kustomize:
$ brew install kustomize
<!-- deploy without using skaffold -->
$ kustomize build deploy/kustomize/overlays/dev/ | kl apply -f -
ref: https://github.com/kubernetes-sigs/kustomize/blob/master/docs/workflows.md
note: `kubectl apply -k` errored for me

Misc Details:
- google auth for container registry: https://cloud.google.com/container-registry/docs/advanced-authentication

Open questions:
- which CD? codefresh? google build run?
- setup canary deploys?
- how do we do rollbacks?
- DR?
- use https://kustomize.io/ instead of helm?
- use skaffold?
