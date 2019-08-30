# k8s the hard way

## Description

The purpose of this code is to learn about kubernetes and how to deploy the Satellite.

Notable differences:
- Secrets are encrypted with gpc kms key and committed to source code
- There is an example using Kustomize instead of Helm
- GKE cluster is used instead of Kops
- docker images are stored on google cloud container registry
- There is an example CI config that automates image builds/pushes and deploys

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
$ kubectl -n kube-system create sa tiller
$ kubectl create clusterrolebinding tiller --clusterrole cluster-admin \
    --serviceaccount=kube-system:tiller
$ helm init --service-account tiller

$ helm install stable/nginx-ingress --name nginx-ingress-ctl \
    --namespace nginx -f infra/base/nginx/nginx.values.yaml

# once nginx ingress controller is up, create a DNS record for 
# Satellite pointing to nginx ingress loadbalancer
# TODO: deploy external-dns to automate ^ this step
```

#### Build Steps

Build the Satellite docker images and push to google cloud container registry. This is also automated in the .circleci/config.yml file.

```
# loging to gcloud
$ gcloud auth loging

# configure [docker to use gcr credentials](https://cloud.google.com/container-registry/docs/advanced-authentication)
$ docker-credential-gcr configure-docker

$ docker build -f deploy/Dockerfile.sa \
    -t gcr.io/storj-jessica/sa-k8s-hard-way/satellite:latest .

$ docker push gcr.io/storj-jessica/sa-k8s-hard-way/satellite:latest
```

#### Deploy Steps

The Satellite can be deployed using Helm or Kustomize. This is also automated in the .circleci/config.yml file.

The secrets that the Satellite needs are encrypted using gcp kms key and [Mozilla SOPs](https://github.com/mozilla/sops#encrypting-using-gcp-kms) and stored in github. In order to deploy the Satellite, first decrypt then create the secrets.

```
$ sops --decrypt deploy/development/sa.env.secret.sops.yaml | kubectl apply -f -

$ sops --decrypt deploy/development/sa.identity.secret.sops.yaml | | kubectl apply -f -
```

Deploy the Satellite with Helm or [Kustomize](https://github.com/kubernetes-sigs/kustomize/blob/master/docs/workflows.md):


```
# Deploy with helm
$ helm install deploy/charts/satellite --name satellite-dev \
    --namespace dev -f deploy/development/sa.values.yaml

# Or deploy with kustomize
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
1. the dockerfile needed to be in root of directory called Dockerfile, I wasn't able to figure out the settings to read from `deploy/Dockerfile.sa` instead.
2. there isn't the ability to provide custom scripts for deploys, therefor we can't decrypt and deploy secrets with the skaffold config

#### Kustomize

I tried out using Kustomize as an alternative to Helm. Kustomize overall seemed to work great as an alternative to Helm's templating.

## Other considerations:
- add liveness/readiness probes to k8s deployment:
https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/
- add external-dns
- Do we want to use terraform
- Should we look into canary deploys or blue/green deploys?
- How to handle rollbacks?
- How to handle DR?
