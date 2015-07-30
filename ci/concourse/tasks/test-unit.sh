#!/usr/bin/env bash

set -e

# NOTE:
#   1. bosh-agent unit tests must be run as non-root (syslog suite).
#   2. at some point, the vcap user may no longer be made available by
#      garden linux. we will need to add that user in our Dockerfile.

sudo chown -R vcap:vcap .
su vcap -c '
  export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$PATH
  export GOPATH=$(pwd)/gopath
  cd gopath/src/github.com/cloudfoundry/bosh-agent
  bin/test-unit
'
