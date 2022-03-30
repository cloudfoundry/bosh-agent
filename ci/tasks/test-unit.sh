#!/usr/bin/env bash
set -eu -o pipefail

CONCOURSE_ROOT="$(pwd)"

chown -R bosh "${CONCOURSE_ROOT}"
# Concourse explicitly changes ownership of the home directory to root, which prevents
# creating go cache directories under there.
chown bosh /home/bosh

cd "${CONCOURSE_ROOT}/bosh-agent"

su bosh -c "env PATH=~bosh/go/bin:$PATH bin/test-unit"
