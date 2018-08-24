package models

type NetboxObject struct {
	ID          uint64 `json:"id"`
	Tags        Tags   `json:"tags"`
	Created     string `json:"created"`
	LastUpdated string `json:"last_updated"`
}

type NetboxCustomFieldsObject struct {
	NetboxObject
	CustomFields CustomFields `json:"custom_fields"`
}

type EmbeddedNetboxObject struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type NetboxList struct {
	Count    uint64 `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
}

type Status struct {
	Value uint   `json:"value"`
	Label string `json:"label"`
}

type Tags []string
type CustomFields map[string]string
