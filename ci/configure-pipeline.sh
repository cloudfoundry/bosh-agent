#!/usr/bin/env bash

set -eu

fly -t production set-pipeline \
	-p bosh-agent:260.x-3312.x \
	-c ci/pipeline.yml \
	-v branch=260.x-3312.x \
	--load-vars-from <(lpass show -G "bosh-agent concourse secrets" --notes)
