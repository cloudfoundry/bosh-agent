FROM ubuntu

# basic deps {
RUN \
  apt-get update \
  && apt-get -y upgrade \
  && apt-get install -y \
    build-essential \
    curl \
    git \
    libssl-dev \
    rsync \
    tar \
    wget \
  && apt-get clean
# }

# vagrant {
# package manager provides 1.4.3, which is too old for vagrant-aws
RUN cd /tmp && wget -q https://releases.hashicorp.com/vagrant/2.2.9/vagrant_2.2.9_x86_64.deb \
 && echo "d06038b68a97a38369220838f0d21c50f9f09b693cf0458cad872864bb7ee09b vagrant_2.2.9_x86_64.deb" | sha256sum -c - \
 && dpkg -i vagrant_2.2.9_x86_64.deb
RUN vagrant plugin install vagrant-aws
# }

COPY --from=golang:1 /usr/local/go /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

RUN useradd -ms /bin/bash bosh
