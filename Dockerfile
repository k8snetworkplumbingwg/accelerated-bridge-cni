FROM golang:1.19-alpine as builder

COPY . /usr/src/accelerated-bridge-cni

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

WORKDIR /usr/src/accelerated-bridge-cni
RUN apk add --no-cache --virtual build-dependencies build-base=~0.5 && \
    make clean && \
    make build

FROM alpine:3
COPY --from=builder /usr/src/accelerated-bridge-cni/build/accelerated-bridge /usr/bin/
WORKDIR /

LABEL io.k8s.display-name="ACCELERATED BRIDGE CNI"

COPY ./images/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
