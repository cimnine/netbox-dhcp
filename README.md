# nine-dhcp2

DHCP Daemon which uses Netbox as datasource and Redis as (local) cache.

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

For now, look at the `nine-dhcp2.conf.yaml` file. It has comments describing all the supported features.

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

## Netbox Assumptions

* There are sites in Netbox. A nine-dhcp2 instance is only responsible for certain sites.
* If interfaces have MAC addresses, then they have not more than one IP assigned.
* If devices have MAC addresses, then they have a primary IP defined.

### Config Context

nine-dhcp2 recognizes the following additional information provided to a Netbox Device via Config Contexts:

```json
{
    "dhcp": {
        "lease_duration": "6h",
        "next_server": "127.0.0.1",
        "bootfile_name": "pxelinux.0",
        "dns_name": "nine.ch",
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

This information takes precedence over what is provided in the nine-dhcp2 config file.
All of the keys are optional.

## Redis

Offered IPs and Leased IPs are added to redis.
If you want persistence, make sure you run Redis in a persisted mode.

nine-dhcp2 uses the following keys to keep track of IPs:

* `v4;{mac};{ip}`
* `v4;{duid};{iaid};{ip}`

## Development

This follows the [go modules][go-modules] introduced with [Go 1.11][go-1.11].

**You will need Go 1.11 or newer!**

[go-modules]: https://golang.org/cmd/go/#hdr-Modules__module_versions__and_more
[go-1.11]: https://golang.org/doc/go1.11

### Local Test Environment

The simplest way to develop the software locally is to use prepared Docker infrastructure.

```bash
docker-compose build
docker-compose up -d
```

#### Fast Local Changes

To avoid building the Docker images all the time, you may build the binary on your host machine directly
and mount the final binary into the container.

You must uncomment the following section in the `docker-compose.yaml` file:

```yaml
# Uncomment for fast development
#    volumes:
#    - ./nine-dhcp2-linux:/app/nine-dhcp2-linux:ro
```

Then compile the binary:

```bash
$ GOOS=linux go build -o nine-dhcp2-linux
$ docker-compose run --rm app /bin/sh
Starting nine-dhcp2_postgres_1 ... done
Starting nine-dhcp2_redis_1    ... done
Starting nine-dhcp2_netbox-worker_1 ... done
Starting nine-dhcp2_netbox_1        ... done
/app # ./nine-dhcp2-linux
``` 

From now on, you can just recompile the binary on your host machine and it will
be synchronized for you to the container by Docker.

#### Load Development Data

Here's how you can load the prepared development data.

```bash
cat dump.sql | docker-compose exec postgres psql -U postgres
```

#### Dump Development Data from Database

This command allows you to update the prepared development data.

```bash
docker-compose exec postgres pg_dump -U netbox --exclude-table-data=extras_objectchange -Cc netbox > dump.sql
```

## Copyright

(c) 2018 Nine Internet Solutions
