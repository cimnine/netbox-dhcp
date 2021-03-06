# netbox-dhcp

DHCP Daemon which uses Netbox as datasource and Redis as (local) cache, written in Go.

**⚠️ WARNING: DEVELOPMENT ONGOING - Totally unstable software right now ⚠️** 

## Roadmap

Main Goal: **Make sure it behaves correctly according to RFC2131 & RFC2132.**

Stretch Goals:

* Serve DHCPv4 through MAC address lookup. ✔
* Serve DHCPv4 through DUID/IAID lookup as described in RFC4361.
* Serve DHCPv6 through MAC address lookup as described in RFC6939.
* Serve DHCPv6 through DUID/IAID lookup.
* Add Prometheus metrics endpoint.
* OpenSource it

## Configuration

For now, look at the `netbox-dhcp.conf.yaml` file. It has comments describing all the supported features.

## DHCP

### Features

* Leases an IP assigned to a Interface based on a MAC lookup for interfaces in Netbox,
  when the Interface has at least 1 IP
* Leases the Device's primary IPv4 based on a MAC lookup for devices in Netbox
* Keep track of leases in a Redis instance
* Supports DHCP release and decline

### Limitations

* ⚠️ NO UNIT TESTS YET ⚠️ --> This is a proof of concept at this stage!

* Responses to Unicast use the MAC the packet was received from instead of doing ARP lookup
* Does not yet support DHCPv4 with client id
* Does not yet implement DHCPINFORM messages
* Does not yet support DHCPv6
* Does not yet support IP pools
* Will not work on non-posix/linux/darwin systems because of the raw socket library

## Netbox Assumptions

* There are sites in Netbox. A netbox-dhcp instance is only responsible for certain sites.
* If interfaces have MAC addresses, then they have not more than one IP assigned.
* If devices have MAC addresses, then they have a primary IP defined.

### Config Context

netbox-dhcp recognizes the following additional information provided to a Netbox Device via Config Contexts:

```json
{
    "dhcp": {
        "lease_duration": "6h",
        "next_server": "127.0.0.1",
        "bootfile_name": "pxelinux.0",
        "dns_name": "cimnine.ch",
        "dns_servers": [
            "1.1.1.1",
            "1.0.0.1"
        ],
        "ntp_servers": [],
        "routers": [
            "172.24.0.1",
            "172.24.0.254"
        ]
    }
}
```

This information takes precedence over what is provided in the netbox-dhcp config file.
All of the keys are optional.

## Redis

Offered IPs and Leased IPs are added to redis.
If you want persistence, make sure you run Redis in a persisted mode.

netbox-dhcp uses the following keys to keep track of IPs:

* `v4;offer;{transactionid};{ip}`, TTL=reservation_duration
* `v4;lease;{mac};{ip}`, TTL=lease_duration
* `v4;lease;{duid};{iaid};{ip}`, TTL=lease_duration

## Development

This follows the [go modules][go-modules] introduced with [Go 1.11][go-1.11].

**You will need Go 1.11 or newer!**

[go-modules]: https://golang.org/cmd/go/#hdr-Modules__module_versions__and_more
[go-1.11]: https://golang.org/doc/go1.11

### Local Environment

There are two local environments, a fully Docker based and a Vagrant-And-Docker based environment.

#### Vagrant

To test, two VMs are prepared for you, a `server` and a `client` machine.
Both will have go installed and the project folder is mounted to `/root/netbox-dhcp`.

First, start both VMs at once:

```bash
vagrant up # starts & provisions a 'server' and a 'client' machine
```

Now connect to the server:

```bash
# in shell 1
vagrant ssh server # connect to the server
/usr/bin/netbox-dhcp # starts the docker dependencies and then execs `go run netbox-dhcp.go`
```

On your workstation go to http://localhost:8080, log in with `admin:admin` and create a site.

In a second terminal window, connect to the client:

```bash
# in shell 2
vagrnat ssh client # connect to the client 
dhclient -v -i enp0s8 # send a dhcp request

# for later:
ping 172.24.0.0.2 # ping the server once a DHCP lease was acquired
dhclient -v -i enp0s8 -r # send a dhcp release
```

Now you should see some output in the `server` shell.
Copy the MAC to your clipboard.
Create a device in Netbox.
Add an interface with that MAC to the device.
Add an IP to the interface (e.g. `172.24.0.10/24`)
Now try to send a _DHCP request_ once more.
The client should assign the IP to the interface.
You should now be able to ping the server: `ping 172.24.0.2` 

When you're done for the day, or done for good, clean up:

```bash
vagrant halt # shutdown but keep state
vagrant destroy # remove machines and destroy state
```

#### Docker

The simplest way to develop the software locally is to use prepared Docker infrastructure.

```bash
docker-compose build
docker-compose up -d
```

But this way is very limited. Currently I'm using two VMs that are on the same private network, and the code directory
is mounted as a shared (read-only) directory.
Eventually this could be turned into a Vagrant setup, so that anyone can quickly fire up a testbed.

###### Fast Local Changes

To avoid building the Docker images all the time, you may build the binary on your host machine directly
and mount the final binary into the container.

You must uncomment the following section in the `docker-compose.yaml` file:

```yaml
# Uncomment for fast development
#    volumes:
#    - ./netbox-dhcp-linux:/app/netbox-dhcp-linux:ro
```

Then compile the binary:

```bash
$ GOOS=linux go build -o netbox-dhcp-linux
$ docker-compose run --rm app /bin/sh
Starting netbox-dhcp_postgres_1 ... done
Starting netbox-dhcp_redis_1    ... done
Starting netbox-dhcp_netbox-worker_1 ... done
Starting netbox-dhcp_netbox_1        ... done
/app # ./netbox-dhcp-linux
``` 

From now on, you can just recompile the binary on your host machine and it will
be synchronized for you to the container by Docker.

###### Load Development Data

Here's how you can load the prepared development data.

```bash
cat dump.sql | docker-compose exec postgres psql -U postgres
```

###### Dump Development Data from Database

This command allows you to update the prepared development data.

```bash
docker-compose exec postgres pg_dump -U netbox --exclude-table-data=extras_objectchange -Cc netbox > dump.sql
```

## Copyright

(c) 2018 Nine Internet Solutions
(c) 2018 Christian Mäder (cimnine)
