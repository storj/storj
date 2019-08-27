# k8s the hard way

## Description

The purpose of this code is to learn about kubernetes and how to deploy the Satellite.

## Requirements

Build Requirements:
- Docker
- Golang

Infrastructure Requirements:
- gcloud
- kubectl

Deploy Requirements:
- [Mozilla SOPs](https://github.com/mozilla/sops)
- [Helm](https://helm.sh/docs/install/) or [Kustomize](https://kustomize.io/)
- [Skaffold](https://github.com/GoogleContainerTools/skaffold) (optional)

## Steps

#### Steps to Create Infrastructure

<!-- TODO: do we want to use terraform for this steps? -->
- Create a [GKE cluster](https://cloud.google.com/kubernetes-engine/docs/quickstart) and a postgres instance with two databases.

```
# loging to gcloud
$ gcloud auth loging

$ gcloud container clusters create k8s-the-hard-way \
    --enable-autoscaling \
    --max-nodes=6 \
    --min-nodes=3

$ gcloud sql instance create k8s-the-hard-way \
    --root-password=$ROOT_PW \

$ gcloud sql databases create master-dev --instance=k8s-the-hard-way
$ gcloud sql databases create metainfo-dev --instance=k8s-the-hard-way
```

- Deploy [Nginx Ingress Controller](https://github.com/helm/charts/tree/master/stable/nginx-ingress) and Helm's Tiller to the GKE cluster

```
$ helm install stable/nginx-ingress --name nginx-ingress-ctl \
    --namespace nginx -f infra/development/nginx.values.yaml

# once nginx ingress controller is up, create a DNS record for Satellite pointing to nginx ingress loadbalancer

$ kubectl -n kube-system create sa tiller
$ kubectl create clusterrolebinding tiller --clusterrole cluster-admin \
    --serviceaccount=kube-system:tiller
$ helm init --service-account tiller
```

#### Build Steps

Build the Satellite docker images and push to google cloud container registry.

```
# loging to gcloud
$ gcloud auth loging

# configure [docker to use gcr credentials](https://cloud.google.com/container-registry/docs/advanced-authentication)
$ docker-credential-gcr configure-docker

$ docker build -f deploy/Dockerfile.sa \
    -t gcr.io/storj-jessica/sa-k8s-hard-way/satellite:latest .

$ docker push gcr.io/storj-jessica/sa-k8s-hard-way/satellite:
```

#### Deploy Steps

The Satellite can be deployed using Helm or Kustomize.

The secrets that the Satellite needs are encrypted using gcp kms key and [Mozilla SOPs](https://github.com/mozilla/sops#encrypting-using-gcp-kms) and stored in github. In order to deploy the Satellite, first decrypt then create the secrets.

```
$ sops --decrypt deploy/development/sa.env.secret.sops.yaml | kubectl apply -f -

$ sops --decrypt deploy/development/sa.identity.secret.sops.yaml | | kubectl apply -f -
```

Deploy the Satellite with Helm or [Kustomize](https://github.com/kubernetes-sigs/kustomize/blob/master/docs/workflows.md):


```
$ helm install deploy/charts/satellite --name satellite-dev \
    --namespace dev -f deploy/development/sa.values.yaml

$ kustomize build deploy/kustomize/overlays/dev/ | kl apply -f -
```

## Experiments:

#### Skaffold
I tested out using [Skaffold for deploys](https://skaffold.dev/docs/how-tos/deployers/#deploying-with-kustomize). The `skaffold.yaml` file in the root of the Storj directory is configured to perform the above build steps and deploy with kustomize or helm to the dev GKE cluster. 

```
$ skaffold config set default-repo gcr.io/storj-jessica/sa-k8s-hard-way

$ skaffold config set --kube-context gke_storj-jessica_us-central1-a_k8s-the-hard-way

$ skaffold run
```

A couple problem I had with skaffold:
1. the dockerfile needed to be in root of directory called Dockerfile, I wasn't able to figure out the seettings to read from `deploy/Dockerfile.sa` instead.
2. there isn't the ability to provide custom scripts for deploys, therefor we can't decrypt and deploy secrets with the skaffold config

#### Kustomize

I tried out using Kustomize as an alternative to Helm. Kustomize overall seemed to work great as an alternative to Helm's templating. 

#### Open issues:
- Do we want to use terraform
- Canary deploys?
- Rollbacks?
- DR?
