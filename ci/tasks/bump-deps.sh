#!/bin/bash

function main() {
  set -e

  git clone bosh-agent bumped-bosh-agent

  mkdir -p workspace/src/github.com/cloudfoundry/
  ln -s $PWD/bumped-bosh-agent workspace/src/github.com/cloudfoundry/bosh-agent

  export GOPATH=$PWD/workspace

  pushd workspace/src/github.com/cloudfoundry/bosh-agent
    dep ensure -v -update

    if [ "$(git status --porcelain)" != "" ]; then
      git status
      git add vendor Gopkg.lock
      git config user.name "CI Bot"
      git config user.email "cf-bosh-eng@pivotal.io"
      git commit -m "Update vendored dependencies"
    fi
  popd
}

main
