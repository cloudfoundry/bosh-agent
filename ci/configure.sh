#!/usr/bin/env bash

fly -t production set-pipeline \
    -p bosh-agent-test-mpdl \
    -c ci/pipeline.yml \
    --load-vars-from <(lpass show -G "bosh-agent concourse secrets" --notes)
