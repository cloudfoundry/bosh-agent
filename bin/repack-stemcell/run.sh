#!/bin/bash

set -e -x

bin=$(dirname "$0")

cd "${bin}"/../..

./bin/build-linux-amd64

mv out/bosh-agent bin/bosh-agent # necessary so that fly -x can be used

git add -f bin/bosh-agent

time fly -t production execute -p -i agent-src=. -o stemcell=out/ -c ./bin/repack-stemcell/task.yml

git reset bin/bosh-agent

ls -la out/stemcell.tgz
