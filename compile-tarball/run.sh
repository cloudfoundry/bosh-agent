#!/usr/bin/env bash

set -exo pipefail

cd compile-tarball || exit 1
  gofumpt -w .
  GOOS=linux GOARCH=amd64 go build .
cd - || exit 1

export TAG
TAG="compile-tarball:0.0.1-dev.$(date +%s)"

docker image list | grep compile-tarbal | awk '{print $1  ":"  $2}' | xargs docker image rm
docker build --platform=linux/amd64 --file ./compile-tarball/Dockerfile -t "${TAG}" .
rm ./compile-tarball/compile-tarball
docker run -v "${PWD}/compile-tarball/spikedata:/releases" --platform=linux/amd64 -it --rm "${TAG}" /bin/compile-tarball --output-directory=/releases /releases/log-cache-release-3.0.7.tgz