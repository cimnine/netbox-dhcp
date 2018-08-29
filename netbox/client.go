package netbox

import (
	"fmt"
	"github.com/ninech/nine-dhcp2/netbox/models"
	"gopkg.in/resty.v1"
	"log"
	"strconv"
)

var emptyParams = map[string]string{}

type EntityResolver interface {
	Resolve() string
}

type Client struct {
	Config *NetboxConfig
}

func (c *Client) GetSites() (res []models.Site, err error) {
	response, err := c.request(emptyParams).
		SetResult(models.SiteList{}).
		Get(c.resolve(models.SiteList{}))

	return response.Result().(*models.SiteList).Sites, err
}

func (c *Client) request(params map[string]string) *resty.Request {
	return resty.R().
		SetQueryParams(params).
		SetHeader("Accept", "application/json").
		SetHeader("Authentication", fmt.Sprintf("Token %s", c.Config.API.Token))
}

func (c Client) resolve(r EntityResolver) string {
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

func (c *Client) FindInterfacesByMac(mac string) (res []models.Interface, err error) {
	// TODO normalize mac address

	// TODO check if resty sanitizes input

	response, err := c.request(map[string]string{
		"mac_address": mac,
	}).SetResult(models.InterfaceList{}).
		Get(c.resolve(models.InterfaceList{}))

	if err != nil {
		log.Printf("An error occurred while receiving interfaces for MAC '%s'", mac)
		return []models.Interface{}, err
	}

	return response.Result().(*models.InterfaceList).Interfaces, err
}

func (c *Client) FindIPAddressesByInterfaceID(ifaceID uint64) ([]models.IP, error) {
	// TODO check if resty sanitizes input

	response, err := c.request(map[string]string{
		"interface_id": strconv.FormatUint(ifaceID, 10),
	}).SetResult(models.IPList{}).
		Get(c.resolve(models.IPList{}))

	if err != nil {
		log.Printf("An error occurred while receiving ips for interface '%s'", ifaceID)
		return []models.IP{}, err
	}

	return response.Result().(*models.IPList).IPs, nil
}
