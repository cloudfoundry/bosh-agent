#!/usr/bin/env bash

absolute_path() {
  (cd "$1" && pwd)
}

scripts_path=$(absolute_path "$(dirname "$0")" )

fly -t production set-pipeline \
    -p bosh-agent:2.91.x \
    -c ${scripts_path}/pipeline.yml \
    --load-vars-from <(lpass show -G "bosh-agent concourse secrets" --notes) \
    --var agent_branch=2.91.x \
    --var version_bump='patch' \
    --var agent_version_key='agent-2.91-current-version' \
    --var agent_initial_version=2.91.0
