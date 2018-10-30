package configuration

import (
	"io/ioutil"
	"log"

	"github.com/cimnine/netbox-dhcp/cache"
	"github.com/cimnine/netbox-dhcp/dhcp/config"
	"github.com/cimnine/netbox-dhcp/netbox"
	"gopkg.in/yaml.v2"
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
