#!/usr/bin/env bash
set -e

## Build docker image
docker build -t mellanox/sriov-cni-test -f ../Dockerfile  ../
