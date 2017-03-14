#!/usr/bin/env bash

set -e

export BASE=$(pwd)
export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$PATH
export GOPATH=$(BASE)/gopath

cd gopath/src/github.com/cloudfoundry/bosh-agent

timestamp=`date -u +"%Y-%m-%dT%H:%M:%SZ"`
git_rev=`git rev-parse --short HEAD`
go_ver=`go version | cut -d ' ' -f 3`
version="${timestamp}-${git_rev}-${go_ver}-${GOOS}-${GOARCH}"

filename="bosh-agent-${version}"

if [[ $GOOS = 'windows' ]]; then
  filename="${filename}.exe"
fi

sed -i 's/\[DEV BUILD\]/'"$version"'/' main/version.go
bin/build

out/bosh-agent -v

sha1sum out/bosh-agent

cp out/bosh-agent "${BASE}/compiled-${GOOS}/${filename}"