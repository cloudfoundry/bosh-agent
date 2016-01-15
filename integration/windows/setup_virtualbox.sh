#!/bin/bash
set -ex

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
OUTPUT_PATH=$DIR/bosh-agent.exe

rm -f $OUTPUT_PATH

GOOS=windows \
  go build \
  -o \
  $OUTPUT_PATH \
  github.com/cloudfoundry/bosh-agent/main

status=$(vagrant global-status | grep bosh-agent)
if echo $status | grep running | grep aws
then
	echo "Vagrant is already running with a different provider"
	exit 1
fi

if echo $status | grep agent | grep running
then
  vagrant provision
else
  vagrant up --provider=virtualbox
fi
