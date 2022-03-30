#!/usr/bin/env bash
set -eu -o pipefail

CONCOURSE_ROOT="$(pwd)"

chown -R bosh "${CONCOURSE_ROOT}"
cd "${CONCOURSE_ROOT}/bosh-agent"

su bosh -c "env PATH=~bosh/go/bin:$PATH GOPATH=$GOPATH bin/test-unit"
