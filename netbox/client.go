package netbox

import (
	"fmt"
	"github.com/ninech/nine-dhcp2/netbox/models"
	"gopkg.in/resty.v1"
	"log"
	"regexp"
	"strconv"
	"strings"
)

type EntityResolver interface {
	Resolve() string
}

type Client struct {
	Config *NetboxConfig
}

func (c *Client) GetSites() (res []models.Site, err error) {
	response, err := c.request().
		SetResult(models.SiteList{}).
		Get(c.resolve(models.SiteList{}))

	return response.Result().(*models.SiteList).Sites, err
}

func (c *Client) request() *resty.Request {
	return resty.R().
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

func (c *Client) FindInterfacesByMAC(mac string) (res []models.Interface, err error) {
	mac = strings.ToUpper(mac)

	if !IsLikelyMAC(mac) {
		log.Printf("'%s' does not seem to be a MAC address!", mac)
	}

	response, err := c.request().
		SetQueryParams(map[string]string{"mac_address": mac}).
		SetResult(models.InterfaceList{}).
		Get(c.resolve(models.InterfaceList{}))

	if err != nil {
		log.Printf("An error occurred while receiving interfaces for MAC '%s'", mac)
		return nil, err
	}

	return response.Result().(*models.InterfaceList).Interfaces, err
}

func (c *Client) FindDevicesByMAC(mac string) (res []models.Device, err error) {
	mac = strings.ToUpper(mac)

	if !IsLikelyMAC(mac) {
		log.Printf("'%s' does not seem to be a MAC address!", mac)
	}

	response, err := c.request().
		SetQueryParams(map[string]string{"mac_address": mac}).
		SetResult(models.DeviceList{}).
		Get(c.resolve(models.DeviceList{}))

	if err != nil {
		log.Printf("An error occurred while receiveing Devices for MAC '%s'", mac)
		return nil, err
	}

	return response.Result().(*models.DeviceList).Devices, nil
}

func (c *Client) GetDeviceByID(id uint64) (res *models.Device, err error) {
	response, err := c.request().
		SetPathParams(map[string]string{"id": strconv.FormatUint(id, 10)}).
		SetResult(models.Device{}).
		Get(c.resolve(models.Device{}))

	if err != nil {
		log.Printf("An error occured while receiveing the Device ID '%d'", id)
		return nil, err
	}

	return response.Result().(*models.Device), nil
}

func (c *Client) GetDeviceByDUID(duid string) (res *models.Device, err error) {
	deviceDUIDField := c.Config.DeviceDUIDField
	response, err := c.request().
		SetPathParams(map[string]string{deviceDUIDField: duid}).
		SetResult(models.Device{}).
		Get(c.resolve(models.Device{}))

	if err != nil {
		log.Printf("An error occured while receiveing the Device by client id: '%s'='%s'", deviceDUIDField, duid)
		return nil, err
	}

	return response.Result().(*models.Device), nil
}

func (c *Client) GetIPAddressByID(id uint64) (res *models.IP, err error) {
	response, err := c.request().
		SetPathParams(map[string]string{"id": strconv.FormatUint(id, 10)}).
		SetResult(models.IP{}).
		Get(c.resolve(models.IP{}))

	if err != nil {
		log.Printf("An error occured while receiveing the IP '%d'", id)
		return nil, err
	}

	return response.Result().(*models.IP), nil
}

func (c *Client) FindIPAddressesByInterfaceID(ifaceID uint64) ([]models.IP, error) {
	response, err := c.request().
		SetQueryParams(map[string]string{"interface_id": strconv.FormatUint(ifaceID, 10)}).
		SetResult(models.IPList{}).
		Get(c.resolve(models.IPList{}))

	if err != nil {
		log.Printf("An error occurred while receiving IPs for Interface '%d'", ifaceID)
		return []models.IP{}, err
	}

	return response.Result().(*models.IPList).IPs, nil
}

func IsLikelyMAC(mac string) (isLikelyMAC bool) {
	isLikelyMAC, err := regexp.MatchString("(?:[a-fA-F0-9]{2}:){5}[a-fA-F0-9]{2}", mac)
	if err != nil {
		log.Fatalf("Regular Expression is wrong: %s", err)
	}

	return
}
