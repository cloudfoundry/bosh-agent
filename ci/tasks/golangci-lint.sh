#!/usr/bin/env bash

set -e

export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$PATH
export GOPATH=$(pwd)/gopath

go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@$GOLANGCI_LINT_VERSION
golangci-lint version
golangci-lint run
