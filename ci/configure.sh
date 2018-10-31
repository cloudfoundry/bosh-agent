#!/usr/bin/env bash

fly -t production set-pipeline \
    -p "bosh-agent:2.160.x" \
    -c pipeline.yml \
    --load-vars-from <(lpass show -G "bosh-agent concourse secrets" --notes) \
    --var agent_branch=2.160.x \
    --var version_bump='patch' \
    --var agent_version_key='agent-2.160-current-version' \
    --var agent_initial_version=2.160.0
