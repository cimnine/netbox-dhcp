package netbox

type NetboxConfig struct {
	API struct {
		URL   string
		Token string
	}
	Cache struct {
		RawDuration string `yaml:"duration"`
	}
	Sites           []string
	DeviceDUIDField string `yaml:"device_duid_field"`
}
