package configuration

import (
	"github.com/ninech/nine-dhcp2/cache"
	"github.com/ninech/nine-dhcp2/dhcp"
	"github.com/ninech/nine-dhcp2/netbox"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

type Configuration struct {
	Netbox netbox.NetboxConfig
	Cache  cache.CacheConfig
	Daemon dhcp.DaemonConfig
	DHCP   dhcp.DHCPConfig `yaml:"dhcp"`
}

func ReadConfig(filename string) (conf Configuration, err error) {
	rawFile, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln("Can't read config file.", err)
		return conf, err
	}

	err = yaml.UnmarshalStrict(rawFile, &conf)
	if err != nil {
		log.Fatalln("Can't parse config file.", err)
		return conf, err
	}

	return conf, err
}
