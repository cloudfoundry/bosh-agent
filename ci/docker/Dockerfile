# Image providing the BOSH-packaged Go toolchain. Overridable so arm64 builds can
# use the arm64 golang-release image (tag "latest-arm64"); the default is the
# amd64 image used today. RFC: https://github.com/cloudfoundry/community/pull/1530
ARG GOLANG_RELEASE_IMAGE=ghcr.io/cloudfoundry/bosh/golang-release
FROM ${GOLANG_RELEASE_IMAGE} AS golang-release

FROM ubuntu
ARG BOSH_CLI_VERSION
ARG CREDHUB_CLI_VERSION
ARG JQ_VERSION

# TARGETARCH is provided automatically by BuildKit (e.g. "amd64", "arm64").
# It lets this image build for both x86_64 and arm64 (Graviton) hosts.
# RFC: cloudfoundry/community#1530.
ARG TARGETARCH

RUN \
  apt-get update \
  && apt-get -y upgrade \
  && apt-get install -y \
    build-essential \
    curl \
    git \
    libssl-dev \
    netcat-openbsd \
    rsync \
    tar \
    wget \
    shellcheck \
  && apt-get clean

# Go is needed both at runtime (agent build/test) and to build the credhub CLI
# from source below, so bring it in before the source-built tooling.
COPY --from=golang-release /var/vcap/packages/golang-1-linux/ /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

RUN wget -nv https://github.com/cloudfoundry/bosh-cli/releases/download/v${BOSH_CLI_VERSION}/bosh-cli-${BOSH_CLI_VERSION}-linux-${TARGETARCH} \
    -O /usr/local/bin/bosh && chmod +x /usr/local/bin/bosh

# jqlang/jq publishes per-arch linux binaries (jq-linux-amd64 / jq-linux-arm64).
# The former stedolan/jq repo is archived and ships no arm64 asset.
RUN wget -nv https://github.com/jqlang/jq/releases/download/${JQ_VERSION}/jq-linux-${TARGETARCH} \
    -O /usr/local/bin/jq && chmod +x /usr/local/bin/jq

# The credhub CLI publishes release binaries for linux-amd64 only, so build it
# from source at the pinned version to get an arch-native binary on arm64.
# Release tags carry no leading "v" (e.g. "2.9.61"), which is not a valid Go
# module version, so build from a shallow clone of the tag rather than
# `go install ...@version`.
RUN git clone --depth 1 --branch "${CREDHUB_CLI_VERSION}" \
      https://github.com/cloudfoundry/credhub-cli.git /tmp/credhub-cli \
    && ( cd /tmp/credhub-cli && go build -o /usr/local/bin/credhub . ) \
    && rm -rf /tmp/credhub-cli

RUN useradd --create-home --shell /bin/bash bosh
