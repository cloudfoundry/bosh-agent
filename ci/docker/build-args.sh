#!/usr/bin/env bash
set -eu

cat << EOF > docker-build-args/docker-build-args.json
{
  "BOSH_CLI_VERSION": "$(cat bosh-cli-github-release/version)",
  "CREDHUB_CLI_VERSION": "$(cat credhub-cli-github-release/version)"
}
EOF

cat docker-build-args/docker-build-args.json
