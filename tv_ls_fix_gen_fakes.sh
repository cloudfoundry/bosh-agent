#!/bin/bash

set -x -e

packages=(
  agent/script/drain/fakes
  agent/script/fakes
  agentclient/fakes
  platform/cert/fakes
)

for f in "${packages[@]}"; do
  (
  cd $f/..
  n=$(echo $f | vim -e +'norm $F/hyiwf/p' +'%p|q!'  /dev/stdin)
  go generate .
  cd -
  rm -rf $f
  find . -name "*_test.go" | grep -v vendor | xargs -n1 sed -i '' "s=$f=$n=g"
  )
done
