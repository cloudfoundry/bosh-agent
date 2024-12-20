#!/usr/bin/env bash

set -e

CONCOURSE_ROOT=$(pwd)

semver=$(cat "${CONCOURSE_ROOT}/version-semver/number")

pushd "${CONCOURSE_ROOT}/bosh-agent"
  git_branch=$(git branch --list -r --contains HEAD | grep -v 'origin/HEAD' | cut -d'/' -f2)
popd

echo "detected bosh-agent will build from branch $git_branch ..."

if [[ "$git_branch" == "main" ]]; then
  echo "SKIPPED: version check is ignored on $git_branch"
  exit 0
fi

version_must_match="^${git_branch//x/[0-9]+}$"
version_must_match="${version_must_match//./\.}"

echo "will only continue if version to promote matches $version_must_match ..."

if ! [[ $semver =~ $version_must_match ]]; then
  echo "version $semver DOES NOT ALIGN with branch $git_branch -- promote step canceled!"
  exit 1
fi

echo "version $semver is appropriate for branch $git_branch -- promote will continue"
