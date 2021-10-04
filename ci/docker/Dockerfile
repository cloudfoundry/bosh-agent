FROM ubuntu
ARG BOSH_CLI_VERSION

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

COPY --from=golang:1.16 /usr/local/go /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

RUN useradd -ms /bin/bash bosh
