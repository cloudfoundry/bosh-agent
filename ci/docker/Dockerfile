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
RUN cd /tmp && wget -q https://releases.hashicorp.com/vagrant/2.0.2/vagrant_2.0.2_x86_64.deb \
 && echo "df8dfb0176d62f0d20d11caec51e53bad57ea2bcc3877427841658702906754f vagrant_2.0.2_x86_64.deb" | sha256sum -c - \
 && dpkg -i vagrant_2.0.2_x86_64.deb
RUN vagrant plugin install vagrant-aws
# }

COPY --from=golang:1 /usr/local/go /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

RUN useradd -ms /bin/bash bosh
