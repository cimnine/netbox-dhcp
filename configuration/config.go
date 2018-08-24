package configuration

import (
	"github.com/ninech/nine-dhcp2/cache"
	"github.com/ninech/nine-dhcp2/dhcp/config"
	"github.com/ninech/nine-dhcp2/netbox"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

type Configuration struct {
	Netbox netbox.NetboxConfig
	Cache  cache.CacheConfig
	Daemon config.DaemonConfig
	DHCP   config.DHCPConfig `yaml:"dhcp"`
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
