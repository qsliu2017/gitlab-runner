#!/bin/bash

# TODO: deprecate this script entirely, replacing with k3s service
# and config chgs in pipline script tags, with actual assertions

if [ "$1" == "" ]
then
    echo Need to specificy a cluster name
    exit 1
fi

# create cluster
gcloud container clusters create $1
kubectl create namespace gitlab-runner
kubectl apply -f gitlab-runner-service-account.yaml
kubectl apply -f gitlab-runner-config.yaml
kubectl apply -f gitlab-runner-deployment.yaml
