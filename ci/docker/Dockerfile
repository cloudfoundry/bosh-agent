FROM bosh/main

RUN /bin/bash -l -c "source /etc/profile.d/chruby.sh ;  \
                     chruby 2.1.7 ;                     \
                     gem install bosh_cli --no-ri --no-rdoc"
