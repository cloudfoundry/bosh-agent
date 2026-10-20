# Image providing the BOSH-packaged Go toolchain. Overridable so arm64 builds can
# use the arm64 golang-release image (tag "latest-arm64"). The default is the
# exact amd64 image used today, so the amd64 build is unchanged.
# RFC: https://github.com/cloudfoundry/community/pull/1530
ARG GOLANG_RELEASE_IMAGE=ghcr.io/cloudfoundry/bosh/golang-release
FROM ${GOLANG_RELEASE_IMAGE} AS golang-release

FROM ubuntu
ARG BOSH_CLI_VERSION
ARG CREDHUB_CLI_VERSION
ARG JQ_VERSION

# TARGETARCH is provided automatically by BuildKit (e.g. "amd64", "arm64") and
# lets this image build for arm64 (Graviton) in addition to amd64. It defaults
# to amd64 so a plain `docker build` without BuildKit reproduces today's image.
# RFC: cloudfoundry/community#1530.
ARG TARGETARCH=amd64

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

RUN wget -nv https://github.com/cloudfoundry/bosh-cli/releases/download/v${BOSH_CLI_VERSION}/bosh-cli-${BOSH_CLI_VERSION}-linux-${TARGETARCH} \
    -O /usr/local/bin/bosh && chmod +x /usr/local/bin/bosh

# amd64 is the unchanged original (stedolan/jq, jq-linux64). stedolan/jq is
# archived and ships no arm64 asset, so arm64 uses the maintained jqlang/jq,
# which publishes jq-linux-arm64. Fail on any other arch rather than silently
# installing the wrong jq. RFC: cloudfoundry/community#1530.
RUN case "${TARGETARCH}" in \
      amd64) wget -nv https://github.com/stedolan/jq/releases/download/${JQ_VERSION}/jq-linux64 \
               -O /usr/local/bin/jq ;; \
      arm64) wget -nv https://github.com/jqlang/jq/releases/download/${JQ_VERSION}/jq-linux-arm64 \
               -O /usr/local/bin/jq ;; \
      *)     echo "Unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && chmod +x /usr/local/bin/jq

COPY --from=golang-release /var/vcap/packages/golang-1-linux/ /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

# amd64 is the unchanged original: download the linux-amd64 credhub CLI release
# tarball. credhub-cli publishes amd64 release binaries only, so arm64 builds the
# CLI from source at the pinned tag with the Go toolchain copied in above. The
# source build passes the same version linker flag the project's Makefile uses,
# so `credhub --version` reports the release version (not an empty string). Fail
# on any other arch rather than silently installing the wrong CLI.
# RFC: cloudfoundry/community#1530.
RUN case "${TARGETARCH}" in \
      amd64) wget -nv https://github.com/cloudfoundry-incubator/credhub-cli/releases/download/${CREDHUB_CLI_VERSION}/credhub-linux-amd64-${CREDHUB_CLI_VERSION}.tgz -O - | \
               tar -zx --directory=/usr/local/bin ;; \
      arm64) git clone --depth 1 --branch "${CREDHUB_CLI_VERSION}" \
               https://github.com/cloudfoundry/credhub-cli.git /tmp/credhub-cli \
             && ( cd /tmp/credhub-cli && go build \
                    -ldflags "-X code.cloudfoundry.org/credhub-cli/version.Version=${CREDHUB_CLI_VERSION}" \
                    -o /usr/local/bin/credhub . ) \
             && rm -rf /tmp/credhub-cli ;; \
      *)     echo "Unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac

RUN useradd --create-home --shell /bin/bash bosh
