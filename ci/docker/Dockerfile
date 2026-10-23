FROM ubuntu@sha256:c098726dd9b9733f3e885f8de0ef375206a311b0a317eadc40d347b1115fb944
ARG BOSH_CLI_VERSION
ARG CREDHUB_CLI_VERSION
ARG JQ_VERSION
# Update these reviewed hashes together with the corresponding tool versions.
# A newly published upstream version must fail verification until reviewed.
# bosh-cli 7.11.2
ARG BOSH_CLI_SHA256=839a92403901639e0e1f3130e4138c912bfd217bd1579277048fcd6bf5dee28c
# jq-1.8.2
ARG JQ_SHA256=b1c22172dd303f3be49e935aa56aa48a8b7a46e0bc838b4997d3bb451495870f
# credhub-cli 2.9.62
ARG CREDHUB_CLI_SHA256=1b933e23d8a531b31ac802fa16d7479e029ac07c6c2598f06a01182ce5fe1001

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


RUN wget -nv https://github.com/cloudfoundry/bosh-cli/releases/download/v${BOSH_CLI_VERSION}/bosh-cli-${BOSH_CLI_VERSION}-linux-amd64 \
    -O /usr/local/bin/bosh \
    && printf '%s  %s\n' "$BOSH_CLI_SHA256" /usr/local/bin/bosh | sha256sum -c - \
    && chmod +x /usr/local/bin/bosh

RUN wget -nv https://github.com/stedolan/jq/releases/download/${JQ_VERSION}/jq-linux64 \
    -O /usr/local/bin/jq \
    && printf '%s  %s\n' "$JQ_SHA256" /usr/local/bin/jq | sha256sum -c - \
    && chmod +x /usr/local/bin/jq

RUN wget -nv https://github.com/cloudfoundry-incubator/credhub-cli/releases/download/${CREDHUB_CLI_VERSION}/credhub-linux-amd64-${CREDHUB_CLI_VERSION}.tgz -O /tmp/credhub.tgz \
    && printf '%s  %s\n' "$CREDHUB_CLI_SHA256" /tmp/credhub.tgz | sha256sum -c - \
    && tar -zxf /tmp/credhub.tgz --directory=/usr/local/bin \
    && rm /tmp/credhub.tgz

COPY --from=ghcr.io/cloudfoundry/bosh/golang-release@sha256:c4c08a9233eda316180fb1ff0a7b653f2a70a3d027514a8777696262f097a356 /var/vcap/packages/golang-1-linux/ /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

RUN useradd --create-home --shell /bin/bash bosh
