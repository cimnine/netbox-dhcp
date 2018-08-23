package netbox

type NetboxConfig struct {
	API struct {
		URL   string
		Token string
	}
	Cache struct {
		RawDuration string `yaml:"duration"`
	}
	Sites []string
}
