Vagrant.configure('2') do |config|
  config.vm.box = 'cloudfoundry/bosh-lite'
  config.vm.box_version = '9000.20.0'

  config.vm.provider :virtualbox do |v, override|
    override.vm.network "private_network", type: "dhcp", id: :local
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

  config.vm.provision :shell, inline: "sudo apt-get update && sudo apt-get install -y jq curl"
  config.vm.provision :shell, inline: "sudo #{agent_dir}/integration/assets/install-go.sh"
  config.vm.provision :shell, inline: "sudo cp #{agent_dir}/integration/assets/bosh-start-logging-and-auditing /var/vcap/bosh/bin/bosh-start-logging-and-auditing"
  config.vm.provision :shell, inline: "sudo #{agent_dir}/integration/assets/disable_growpart.sh"
end
