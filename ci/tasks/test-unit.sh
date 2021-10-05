#!/usr/bin/env bash

set -e

export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$PATH
export GOPATH=$(pwd)/gopath
chown -R bosh .
cd gopath/src/github.com/cloudfoundry/bosh-agent
mkdir -p /home/bosh
chown bosh:bosh /home/bosh
su bosh -c "env PATH=$PATH GOPATH=$GOPATH bin/test-unit"
