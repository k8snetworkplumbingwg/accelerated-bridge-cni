#!/usr/bin/env bash
set -e

## Build docker image
docker build -t mellanox/accelerated-bridge-cni-test -f ../Dockerfile  ../
