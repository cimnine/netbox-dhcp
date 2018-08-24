# nine-dhcp2

DHCP Daemon which uses Netbox as datasource.

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
