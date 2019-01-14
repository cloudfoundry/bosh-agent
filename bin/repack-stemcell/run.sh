#!/bin/bash

set -e -x

bin=$(dirname "$0")

cd "${bin}"/../..

./bin/build-linux-amd64

tempdir=$(mktemp -d)
function cleanup {
  rm -rf "$tempdir"
}
trap cleanup EXIT


mv out/bosh-agent ${tempdir}/bosh-agent

time fly -t production execute -p -i compiled-agent=${tempdir} -i agent-src=. -o stemcell=out/ -c ./bin/repack-stemcell/task.yml

ls -la out/stemcell.tgz
