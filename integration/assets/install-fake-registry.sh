#!/usr/bin/env bash

set -ex

GOPATH=/home/vagrant/go
export GOROOT=/usr/local/go
export PATH=$GOROOT/bin:$PATH

base=$( cd $(dirname $0)/../.. && pwd )
bin=$base/bin

goversion=`$bin/go version | awk '{print $3}'`

if [ $goversion != "go1.3.3" ]
then
  echo "Currently using go version $goversion, must be using go1.3.3"
  exit 1
fi

$bin/go build -o $base/tmp/fake-registry github.com/cloudfoundry/bosh-agent/integration/fake-registry