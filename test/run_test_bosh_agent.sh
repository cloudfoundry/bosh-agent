#!/bin/bash
../out/bosh-agent  -b $PWD -P dummy -M dummy-nats -M dummy -C agent.json
