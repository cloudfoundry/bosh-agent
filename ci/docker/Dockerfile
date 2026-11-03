ARG GOLANG_RELEASE_IMAGE=ghcr.io/cloudfoundry/bosh/golang-release
FROM ${GOLANG_RELEASE_IMAGE} AS golang-release

FROM ubuntu
ARG BOSH_CLI_VERSION
ARG CREDHUB_CLI_VERSION
ARG JQ_VERSION

# No default: a default would override the value BuildKit sets from the target
# platform. Without BuildKit this is empty and the arch checks below fail.
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

RUN wget -nv https://github.com/cloudfoundry/bosh-cli/releases/download/v${BOSH_CLI_VERSION}/bosh-cli-${BOSH_CLI_VERSION}-linux-${TARGETARCH} \
    -O /usr/local/bin/bosh && chmod +x /usr/local/bin/bosh

# stedolan/jq is archived and ships no arm64 asset, so arm64 uses jqlang/jq.
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

# credhub-cli publishes amd64 release binaries only, so arm64 builds from source.
# The version linker flag matches the upstream Makefile.
RUN case "${TARGETARCH}" in \
      amd64) wget -nv https://github.com/cloudfoundry-incubator/credhub-cli/releases/download/${CREDHUB_CLI_VERSION}/credhub-linux-amd64-${CREDHUB_CLI_VERSION}.tgz -O - | \
               tar -zx --directory=/usr/local/bin ;; \
      arm64) git clone --depth 1 --branch "${CREDHUB_CLI_VERSION}" \
               https://github.com/cloudfoundry/credhub-cli.git /tmp/credhub-cli \
             && ( cd /tmp/credhub-cli && go build \
                    -ldflags "-X code.cloudfoundry.org/credhub-cli/version.Version=${CREDHUB_CLI_VERSION}" \
                    -o /usr/local/bin/credhub . ) \
             && rm -rf /tmp/credhub-cli \
             && go clean -cache -modcache ;; \
      *)     echo "Unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac

RUN useradd --create-home --shell /bin/bash bosh
