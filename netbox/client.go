package netbox

import (
	"fmt"
	"github.com/ninech/nine-dhcp2/netbox/models"
	"gopkg.in/resty.v1"
	"log"
	"strconv"
)

type Resolver interface {
	Resolve() string
}

type Client struct {
	Config *NetboxConfig
}

func (c *Client) GetSites() (res []models.Site, err error) {
	response, err := c.request(map[string]string{}).
		SetResult(models.SiteList{}).
		Get(c.resolve(models.Site{}))

	return response.Result().(*models.SiteList).Sites, err
}

func (c *Client) request(params map[string]string) *resty.Request {
	return resty.R().
		SetQueryParams(params).
		SetHeader("Accept", "application/json").
		SetHeader("Authentication", fmt.Sprintf("Token %s", c.Config.API.Token))
}

func (c Client) resolve(r Resolver) string {
	return c.Config.API.URL + r.Resolve()
}

func (c *Client) CheckSites() bool {
	sites, err := c.GetSites()
	if err != nil {
		log.Fatalln("Can't fetch Sites from Netbox", err)
		return false
	}

	sitesCheck := make(map[string]bool)
	for _, s := range c.Config.Sites {
		sitesCheck[s] = false
	}

	for _, s := range sites {
		if s.Status.Value == 1 {
			siteID := strconv.FormatUint(s.ID, 10)
			sitesCheck[siteID] = true
		}
	}

	allGood := true
	for siteID, found := range sitesCheck {
		if !found {
			log.Printf("Site '%s' not found or it's inactive.", siteID)
			allGood = false
		}
	}

	return allGood
}
