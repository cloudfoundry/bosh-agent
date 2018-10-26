FROM ubuntu

# basic deps {
RUN \
  apt-get update \
  && apt-get -y upgrade \
  && apt-get install -y \
    build-essential \
    curl \
    git \
    jq \
    libssl-dev \
    rsync \
    tar \
    wget \
  && apt-get clean
# }

RUN wget -O /usr/bin/meta4 https://github.com/dpb587/metalink/releases/download/v0.2.0/meta4-0.2.0-linux-amd64 \
  && echo "81a592eaf647358563f296aced845ac60d9061a45b30b852d1c3f3674720fe19  /usr/bin/meta4" | shasum -a 256 -c \
  && chmod +x /usr/bin/meta4

# vagrant {
# package manager provides 1.4.3, which is too old for vagrant-aws
RUN cd /tmp && wget -q https://releases.hashicorp.com/vagrant/2.0.2/vagrant_2.0.2_x86_64.deb \
 && echo "df8dfb0176d62f0d20d11caec51e53bad57ea2bcc3877427841658702906754f vagrant_2.0.2_x86_64.deb" | sha256sum -c - \
 && dpkg -i vagrant_2.0.2_x86_64.deb
RUN vagrant plugin install vagrant-aws
# }

COPY --from=golang:1 /usr/local/go /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH
