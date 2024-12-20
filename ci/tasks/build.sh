#!/usr/bin/env bash

set -e

CONCOURSE_ROOT=$(pwd)

semver=$(cat "${CONCOURSE_ROOT}/version-semver/number")

filename_suffix="${semver}-${GOOS}-${GOARCH}"
if [[ $GOOS = 'windows' ]]; then
  filename_suffix="${filename_suffix}.exe"
fi

cd bosh-agent

git_rev=$(git rev-parse --short HEAD)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
go_ver=$(go version | cut -d ' ' -f 3)
version="${semver}-${git_rev}-${timestamp}-${go_ver}"

export VERSION_LABEL="${version}"

bin/build

# output bosh-agent
shasum -a 256 out/bosh-agent
cp out/bosh-agent "${CONCOURSE_ROOT}/${OUTPUT_DIR}/bosh-agent-${filename_suffix}"

if [[ $GOOS = 'windows' ]]; then
  shasum -a 256 out/bosh-agent-pipe
  cp out/bosh-agent-pipe "${CONCOURSE_ROOT}/${OUTPUT_DIR}/bosh-agent-pipe-${filename_suffix}"

  shasum -a 256 integration/windows/fixtures/service_wrapper.xml
  cp integration/windows/fixtures/service_wrapper.xml "${CONCOURSE_ROOT}/${OUTPUT_DIR}"

  echo "${git_rev}" > "${CONCOURSE_ROOT}/${OUTPUT_DIR}/git-sha"
fi
