#!/usr/bin/env bash

set -eu

fly -t production set-pipeline \
	-p bosh-agent:261.x-3363.x \
	-c ci/pipeline.yml \
	-v branch=261.x-3363.x \
	--load-vars-from <(lpass show -G "bosh-agent concourse secrets" --notes)
