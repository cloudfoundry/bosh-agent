FROM ubuntu:14.04

RUN apt-get update
RUN apt-get -y upgrade && apt-get clean
RUN apt-get install -y \
  build-essential \
  wget \
  tar \
  libssl-dev \
  && apt-get clean

ADD install-go.sh /tmp/install-go.sh
RUN chmod a+x /tmp/install-go.sh

ENV GOROOT /usr/local/go
ENV PATH $GOROOT/bin:$PATH
RUN cd /tmp && ./install-go.sh && rm install-go.sh

