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

ADD install-go.sh /tmp/install-go.sh
RUN chmod a+x /tmp/install-go.sh

ENV GOROOT /usr/local/go
ENV PATH $GOROOT/bin:$PATH
RUN cd /tmp && ./install-go.sh && rm install-go.sh

