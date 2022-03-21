#!/usr/bin/env bash

set -e

export GOPATH
GOPATH="$(pwd)/gopath"
export PATH=/usr/local/ruby/bin:/usr/local/go/bin:${GOPATH}/bin:${PATH}

cd gopath/src/github.com/cloudfoundry/bosh-agent

bin/golangci-lint
