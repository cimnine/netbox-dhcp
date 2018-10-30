# -*- mode: ruby -*-
# vi: set ft=ruby :

# All Vagrant configuration is done below. The "2" in Vagrant.configure
# configures the configuration version (we support older styles for
# backwards compatibility). Please don't change it unless you know what
# you're doing.
Vagrant.configure('2') do |config|
  # The most common configuration options are documented and commented below.
  # For a complete reference, please see the online documentation at
  # https://docs.vagrantup.com.

  # Every Vagrant development environment requires a box. You can search for
  # boxes at https://vagrantcloud.com/search.
  config.vm.define 'server' do |server|
    server.vm.box = 'ubuntu/bionic64'
    server.vm.network 'private_network', ip: '172.24.0.2', virtualbox__intnet: 'nine-dhcp2'
    server.vm.network 'forwarded_port', guest: 8080, host: 8080
    server.vm.network 'forwarded_port', guest: 6379, host: 6379
    server.vm.provision 'shell', inline: <<~SHELL
      apt-get install -y \
        dnsmasq \
        apt-transport-https \
        ca-certificates \
        curl \
        software-properties-common
      curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
      add-apt-repository \
        "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
        $(lsb_release -cs) \
        stable"
      apt-get update
      apt-get install -y docker-ce
      curl -sS -L \
        "https://github.com/docker/compose/releases/download/1.22.0/docker-compose-$(uname -s)-$(uname -m)" \
        -o /usr/local/bin/docker-compose
      chmod +x /usr/local/bin/docker-compose
      curl -sS -L \
        "https://raw.githubusercontent.com/docker/compose/1.22.0/contrib/completion/bash/docker-compose" \
        -o /etc/bash_completion.d/docker-compose
      ln -s /root/nine-dhcp2/nine-dhcp2.vagrant.conf.yaml /etc/nine-dhcp2.conf.yaml
      echo 'alias docker-compose="docker-compose -f docker-compose.vagrant.yaml"' >> "/root/.bashrc"
      echo 'alias dc="docker-compose"' >> "/root/.bashrc"
      cat <<SETUP > /usr/bin/nine-dhcp2
      #!/bin/bash
      set +e
      if [ "$(id -u)" != "0" ] || [ "${HOME}" != "/root" ]; then
        exec sudo -i $0
      fi
      cd /root/nine-dhcp2
      docker-compose -f docker-compose.vagrant.yaml up -d
      if [ "${INSIDE}" != "1" ]; then
        export INSIDE=1
        go run nine-dhcp2.go
        exec bash
      else
        exec go run nine-dhcp2.go
      fi
      SETUP
      chmod a+x /usr/bin/nine-dhcp2
      {
        cd /root/nine-dhcp2
        docker-compose -f docker-compose.vagrant.yaml pull
      }
    SHELL
  end

  config.vm.define 'client' do |client|
    client.vm.box = 'ubuntu/bionic64'
    client.vm.network 'private_network', ip: '172.24.0.10', auto_config: false, virtualbox__intnet: 'nine-dhcp2'
    client.vm.provision 'shell', inline: <<-SHELL
      apt-get install -y isc-dhcp-client
    SHELL
  end

  # Disable automatic box update checking. If you disable this, then
  # boxes will only be checked for updates when the user runs
  # `vagrant box outdated`. This is not recommended.
  # config.vm.box_check_update = false

  # Create a forwarded port mapping which allows access to a specific port
  # within the machine from a port on the host machine. In the example below,
  # accessing "localhost:8080" will access port 80 on the guest machine.
  # NOTE: This will enable public access to the opened port
  # config.vm.network "forwarded_port", guest: 80, host: 8080

  # Create a forwarded port mapping which allows access to a specific port
  # within the machine from a port on the host machine and only allow access
  # via 127.0.0.1 to disable public access
  # config.vm.network "forwarded_port", guest: 80, host: 8080, host_ip: "127.0.0.1"

  # Create a private network, which allows host-only access to the machine
  # using a specific IP.
  # config.vm.network "private_network", ip: "192.168.33.10"

  # Create a public network, which generally matched to bridged network.
  # Bridged networks make the machine appear as another physical device on
  # your network.
  # config.vm.network "public_network"

  # Share an additional folder to the guest VM. The first argument is
  # the path on the host to the actual folder. The second argument is
  # the path on the guest to mount the folder. And the optional third
  # argument is a set of non-required options.
  config.vm.synced_folder '.', '/root/nine-dhcp2', owner: 'root', group: 'root', mount_options: ['ro']

  # Provider-specific configuration so you can fine-tune various
  # backing providers for Vagrant. These expose provider-specific options.
  # Example for VirtualBox:
  #
  config.vm.provider 'virtualbox' do |vb|
  #   # Display the VirtualBox GUI when booting the machine
  #   vb.gui = true
  #
    # Customize the amount of memory on the VM:
    vb.memory = '1024'

    # Don't copy base box around, just use different disks
    vb.linked_clone = true
  end
  #
  # View the documentation for the provider you are using for more
  # information on available options.

  # Enable provisioning with a shell script. Additional provisioners such as
  # Puppet, Chef, Ansible, Salt, and Docker are also available. Please see the
  # documentation for more information about their specific syntax and use.
  config.vm.provision 'shell', inline: <<-SHELL
    apt-get update
    apt-get install -y \
      build-essential
    wget -nv "https://godeb.s3.amazonaws.com/godeb-amd64.tar.gz"
    tar xzf godeb-amd64.tar.gz
    ./godeb install 1.11
  SHELL
end
