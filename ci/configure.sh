#!/usr/bin/env bash

absolute_path() {
  (cd "$1" && pwd)
}

scripts_path=$(absolute_path "$(dirname "$0")")

fly -t "${CONCOURSE_TARGET:-bosh-ecosystem}" set-pipeline \
    -p bosh-agent-main \
    -c $scripts_path/pipeline.yml
