FROM ubuntu:14.04

RUN apt-get update
RUN apt-get -y upgrade && apt-get clean
RUN apt-get install -y \
  build-essential \
  git \
  wget \
  tar \
  libssl-dev \
  && apt-get clean

# package manager provides 1.4.3, which is too old for vagrant-aws
RUN cd /tmp && wget -q https://releases.hashicorp.com/vagrant/2.0.2/vagrant_2.0.2_x86_64.deb \
 && echo "df8dfb0176d62f0d20d11caec51e53bad57ea2bcc3877427841658702906754f vagrant_2.0.2_x86_64.deb" | sha256sum -c - \
 && dpkg -i vagrant_2.0.2_x86_64.deb
RUN vagrant plugin install vagrant-aws

# golang {
ARG GO_VERSION=1.9.2
ARG GO_SHA=de874549d9a8d8d8062be05808509c09a88a248e77ec14eb77453530829ac02b

ENV GO_TAR=go$GO_VERSION.linux-amd64.tar.gz

WORKDIR /usr/local

ENV \
  PATH=/usr/local/go/bin:$PATH \
  GOPATH=/root/go \
  GOROOT=/usr/local/go

RUN curl -fSL https://storage.googleapis.com/golang/$GO_TAR -o $GO_TAR \
  && echo $GO_SHA $GO_TAR | sha256sum -c - \
  && tar -xzf $GO_TAR
# }
