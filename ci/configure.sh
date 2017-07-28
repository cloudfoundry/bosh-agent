#!/usr/bin/env bash

fly -t production set-pipeline \
    -p bosh-agent:2.1.x \
    -c pipeline.yml \
    --load-vars-from <(lpass show -G "bosh-agent concourse secrets" --notes)
