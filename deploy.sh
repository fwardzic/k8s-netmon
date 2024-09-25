#!/bin/sh

docker build -t k8s-netmon:v0.1 .
kind load docker-image k8s-netmon:v0.1
kubectl rollout restart daemonset -n netmon k8s-netmon