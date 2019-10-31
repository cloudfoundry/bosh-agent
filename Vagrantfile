Vagrant.configure('2') do |config|
  config.vm.box = "xcoo/xenial64"
  config.vm.hostname = 'bosh-agent-integration-tests'

  config.vm.provider :virtualbox do |v, override|
    override.vm.network "private_network", type: "dhcp", id: :local
    v.customize ['modifyvm', :id, '--natdnshostresolver1', 'on']
    v.customize ['modifyvm', :id, '--natdnsproxy1', 'on']
    v.customize ['modifyvm', :id, '--paravirtprovider', 'minimal']
  end

  config.vm.provider :aws do |v, override|
    v.associate_public_ip = true
    # To turn off public IP echoing, uncomment this line:
    # override.vm.provision :shell, id: "public_ip", run: "always", inline: "/bin/true"

    # To turn off CF port forwarding, uncomment this line:
    # override.vm.provision :shell, id: "port_forwarding", run: "always", inline: "/bin/true"
    v.tags = {
      'PipelineName' => 'bosh-agent'
    }

    v.access_key_id = ENV['BOSH_AWS_ACCESS_KEY_ID'] || ''
    v.secret_access_key = ENV['BOSH_AWS_SECRET_ACCESS_KEY'] || ''
    v.subnet_id = ENV['BOSH_LITE_SUBNET_ID'] || ''
    v.ami = ''
    v.access_key_id =       ENV.fetch('BOSH_AWS_ACCESS_KEY_ID', nil)
    v.secret_access_key =   ENV.fetch('BOSH_AWS_SECRET_ACCESS_KEY', nil)
    v.region =              ENV.fetch('BOSH_LITE_REGION', 'us-east-1')
    v.keypair_name =        ENV.fetch('BOSH_LITE_KEYPAIR', 'bosh')
    v.instance_type =       ENV.fetch('BOSH_LITE_INSTANCE_TYPE', 'm3.xlarge')
    v.block_device_mapping = [{
      :DeviceName => '/dev/sda1',
      'Ebs.VolumeType' => 'gp2',
      'Ebs.VolumeSize' => ENV.fetch('BOSH_LITE_DISK_SIZE', '80').to_i
    }]
    v.security_groups =     [ENV.fetch('BOSH_LITE_SECURITY_GROUP', 'inception')]
    v.subnet_id =           ENV.fetch('BOSH_LITE_SUBNET_ID') if ENV.include?('BOSH_LITE_SUBNET_ID')
    v.private_ip_address =  ENV.fetch('BOSH_LITE_PRIVATE_IP') if ENV.include?('BOSH_LITE_PRIVATE_IP')
  end
  agent_dir = '/home/vagrant/go/src/github.com/cloudfoundry/bosh-agent'

  # We need to override the rsync args to exlucde "--copy-links".
  # This is due to the fact that `dep` does not prune symlinks from the vendor directory.
  # A vendored dependency has a broken symlink for test, which breaks `rsync`.
  # See https://github.com/golang/dep/issues/1625 for more context
  config.vm.synced_folder '.', agent_dir, type: "rsync",
    rsync__args: ["--verbose", "--archive", "--delete", "-z"]

#  config.vm.synced_folder Dir.pwd, '/vagrant', disabled: true
  config.vm.provision :shell, inline: "mkdir -p /vagrant && chmod 777 /vagrant"

  config.vm.provision :shell, inline: <<-SHELL
    rm -f /var/lib/dpkg/lock
    rm -f /var/lib/dpkg/lock-frontend
    dpkg --configure -a
    echo $(hostname -I) $(hostname) | tee -a /etc/hosts
    apt-get update && apt-get install -y jq curl iputils-arping runit
    groupadd -f vcap
    useradd -m --comment 'BOSH System User' vcap --uid 1002 -g vcap || true
    groupadd -f --system admin
    groupadd -f bosh_sshers
    groupadd -f bosh_sudoers
    usermod vcap -a -G vagrant
    usermod vagrant -a -G vcap
    mkdir -p /var/vcap/bosh/bin
    mkdir -p /etc/service/agent
    mkdir -p /etc/sv/monit
    mkdir -p /var/vcap/monit/svlog
    mkdir -p /var/log
    touch /var/log/monit.log
    mkdir -p /var/vcap/bosh/log
    mkdir -p /var/vcap/bosh/etc
    mkdir -p /var/vcap/jobs
    chmod 0755 /var/vcap/jobs
    touch /var/vcap/monit/empty.monitrc

    #{agent_dir}/integration/assets/install-go.sh
    #{agent_dir}/integration/assets/install-agent.sh
    #{agent_dir}/integration/assets/install-fake-registry.sh
    #{agent_dir}/integration/assets/install-fake-blobstore.sh
    cp -a #{agent_dir}/integration/assets/alerts.monitrc /var/vcap/monit/alerts.monitrc
    chmod 0600 /var/vcap/monit/alerts.monitrc
    chown root:root /var/vcap/monit/alerts.monitrc
    cp -r #{agent_dir}/integration/assets/runit/monit/* /etc/sv/monit
    cp -r #{agent_dir}/integration/assets/runit/agent/* /etc/service/agent
    cp -r #{agent_dir}/integration/assets/agent_runit.sh /etc/service/agent/run
    cp -r #{agent_dir}/integration/assets/generic_settings.json /var/vcap/bosh/settings.json

    cp #{agent_dir}/integration/assets/monit /var/vcap/bosh/bin/monit
    cp #{agent_dir}/integration/assets/monitrc /var/vcap/bosh/etc/monitrc
    chmod 0700 /var/vcap/bosh/etc/monitrc

    cp #{agent_dir}/integration/assets/bosh-start-logging-and-auditing /var/vcap/bosh/bin/bosh-start-logging-and-auditing
    cp #{agent_dir}/integration/assets/bosh-agent-rc /var/vcap/bosh/bin/bosh-agent-rc

    systemctl restart runit
SHELL

  config.vm.provision :shell, inline: "#{agent_dir}/integration/assets/disable_growpart.sh"
  config.vm.provision :shell, inline: "echo '#!/bin/bash' > /var/vcap/bosh/bin/restart_networking"
  config.vm.provision :shell, inline: "chmod +x /var/vcap/bosh/bin/restart_networking"
  config.vm.provision :shell, inline: "mkdir -p /etc/systemd/network/"
end
