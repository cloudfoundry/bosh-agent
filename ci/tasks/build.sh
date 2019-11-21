#!/usr/bin/env bash

set -e

export BASE=$(pwd)
export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$PATH
export GOPATH=${BASE}/gopath

semver=`cat ${BASE}/version-semver/number`

goversion_suffix=""
if [ ! -z "${GOVERSION}" ]; then
  goversion_suffix="-${GOVERSION}"
fi

filename_suffix="${semver}${goversion_suffix}-${GOOS}-${GOARCH}"
if [[ $GOOS = 'windows' ]]; then
  filename_suffix="${filename_suffix}.exe"
fi

timestamp=`date -u +"%Y-%m-%dT%H:%M:%SZ"`
go_ver=`go version | cut -d ' ' -f 3`

cd gopath/src/github.com/cloudfoundry/bosh-agent

git_rev=`git rev-parse --short HEAD`

version="${semver}-${git_rev}-${timestamp}-${go_ver}"
sed -i 's/\[DEV BUILD\]/'"$version"'/' main/version.go

bin/build

# output bosh-agent
shasum -a 256 out/bosh-agent
cp out/bosh-agent "${BASE}/${DIRNAME}/bosh-agent-${filename_suffix}"

if [[ $GOOS = 'windows' ]]; then
  shasum -a 256 out/bosh-agent-pipe
  cp out/bosh-agent-pipe "${BASE}/${DIRNAME}/bosh-agent-pipe-${filename_suffix}"
fi
