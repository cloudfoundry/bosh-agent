FROM ubuntu
ARG BOSH_CLI_VERSION
ARG CREDHUB_CLI_VERSION
ARG JQ_VERSION

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
  && apt-get clean


RUN wget -nv https://github.com/cloudfoundry/bosh-cli/releases/download/v${BOSH_CLI_VERSION}/bosh-cli-${BOSH_CLI_VERSION}-linux-amd64 \
    -O /usr/local/bin/bosh && chmod +x /usr/local/bin/bosh

RUN wget -nv https://github.com/stedolan/jq/releases/download/${JQ_VERSION}/jq-linux64 \
    -O /usr/local/bin/jq && chmod +x /usr/local/bin/jq

RUN wget -nv https://github.com/cloudfoundry-incubator/credhub-cli/releases/download/${CREDHUB_CLI_VERSION}/credhub-linux-${CREDHUB_CLI_VERSION}.tgz -O - | \
    tar -zx --directory=/usr/local/bin

COPY --from=bosh/golang-release /var/vcap/packages/golang-1-linux/ /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

RUN useradd --create-home --shell /bin/bash bosh
