#!/usr/bin/env bash
set -e

## Build docker image
docker build -t k8snetworkplumbingwg/accelerated-bridge-cni-test -f ../Dockerfile  ../
