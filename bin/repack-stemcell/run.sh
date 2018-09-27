#!/bin/bash

set -e -x

bin=$(dirname "$0")

cd "${bin}"/../..

./bin/build-linux-amd64

mv out/bosh-agent bin/bosh-agent # necessary so that fly -x can be used

time fly -t aws execute -p -i agent-src=/Users/cpi/go/src/github.com/cloudfoundry/bosh-agent -o stemcell=/Users/cpi/go/src/github.com/cloudfoundry/bosh-agent/out -c ./bin/repack-stemcell/task.yml

ls -la out/stemcell.tgz
