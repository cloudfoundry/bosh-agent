#!/usr/bin/env bash

absolute_path() {
  (cd "$1" && pwd)
}

scripts_path=$(absolute_path "$(dirname "$0")")

fly -t production set-pipeline \
    -p bosh-agent:2.234.x \
    -c $scripts_path/pipeline.yml \
    --load-vars-from <(lpass show -G "bosh-agent concourse secrets" --notes)
