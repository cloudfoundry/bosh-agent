#!/usr/bin/env bash

absolute_path() {
  (cd "$1" && pwd)
}

scripts_path=$(absolute_path "$(dirname "$0")")

fly -t bosh-ecosystem set-pipeline \
    -p bosh-agent-2.268.x \
    -c $scripts_path/pipeline.yml
