#!/usr/bin/env bash

set -ex

# golang {
pushd /usr/local
  curl -Lo /usr/local/bin/jq https://github.com/stedolan/jq/releases/download/jq-1.5/jq-linux64
  chmod +x /usr/local/bin/jq
  GO_INFO=$(
    curl -s 'https://golang.org/dl/?mode=json' | \
      jq 'map(select(.stable and (.version | split(".")[0] == "go1"))) | .[0].files[] | select(.os == "linux" and .arch == "amd64")'
  )
  GO_TAR="$(echo "$GO_INFO" | jq -r '.filename')"
  GO_SHA="$(echo "$GO_INFO" | jq -r '.sha256')"
  curl -fSL https://storage.googleapis.com/golang/$GO_TAR -o $GO_TAR
  echo $GO_SHA $GO_TAR | sha256sum -c -
  tar -xzf $GO_TAR
  rm -f $GO_TAR

  export PATH=/usr/local/go/bin:$PATH
  export GOPATH=/home/vagrant/go
  export GOROOT=/usr/local/go
popd
# }

chmod -R a+w $GOROOT

if [ ! -d $TMPDIR ]; then
  mkdir -p $TMPDIR
fi
