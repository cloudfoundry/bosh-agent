#!/usr/bin/env bash

set -e

export GOPATH=$(pwd)/gopath
export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$GOPATH/bin:$PATH

cd gopath/src/github.com/cloudfoundry/bosh-agent

go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@$GOLANGCI_LINT_VERSION
golangci-lint version
golangci-lint run
