package resolver

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/ninech/nine-dhcp2/dhcp/config"
	"log"
	"time"
)

type Redis struct {
	Client     *redis.Client
	DHCPConfig *config.DHCPConfig
}

// REDIS STRUCTURE
// -------------------------------------
// key:						      					value:
// -------------------------------------
// v4;{mac};{ip}									{json}
// v4;{duid};{iaid};{ip}					{json}
// -------------------------------------

func (r Redis) AcknowledgeV4ByMAC(mac, ip string) (*ClientInfoV4, error) {
	key := keyMAC(4, mac, ip)

	log.Printf("Receiving info about '%s' from cache.", key)

	result := r.Client.Get(key)
	if result.Err() != nil {
		log.Printf("Unable to receive info about '%s': %s", key, result.Err())
		return nil, result.Err()
	}

	rawInfo, err := result.Bytes()
	if err != nil {
		log.Printf("Unable to extract info from '%s': %s", key, err)
		return nil, err
	}

	info := ClientInfoV4{}
	err = json.Unmarshal(rawInfo, &info)
	if err != nil {
		log.Printf("Unable to reconstruct info from '%s': %s", key, err)
		return nil, err
	}

	r.Client.Set(key, rawInfo, info.Timeouts.Lease)

	return &info, nil
}

func (r Redis) AcknowledgeV4ByID(duid, iaid, ip string) (*ClientInfoV4, error) {
	panic("implement me")
}

func (r Redis) ReserveV4ByMAC(mac string, info *ClientInfoV4) error {
	r.calculateTimeouts(info)

	infoAsJson, err := json.Marshal(info)
	if err != nil {
		log.Printf("Can't convert payload for MAC '%s': %s", mac, err)
		return err
	}

	key := keyMAC(4, mac, info.IPAddr.String())

	log.Printf("Writing info about '%s' to the cache.", key)

	status := r.Client.Set(key, infoAsJson, info.Timeouts.Reservation)
	if status.Err() != nil {
		log.Printf("Can't add info for '%s' to the cache: %s", key, status.Err())
		return status.Err()
	}

	log.Printf("Wrote info about '%s' to the cache.", key)

	return nil
}

func (r Redis) ReserveV4ByID(duid, iaid string, info *ClientInfoV4) error {
	r.calculateTimeouts(info)
	panic("implement me")
}

func keyMAC(familiy uint8, mac, ip string) string {
	return fmt.Sprintf("v%d;%s;%s", familiy, mac, ip)
}

func (r Redis) calculateTimeouts(info *ClientInfoV4) {
	if info.Timeouts.Reservation == 0 {
		d, err := time.ParseDuration(r.DHCPConfig.ReservationTimeout)
		if err != nil {
			info.Timeouts.Reservation = 1 * time.Minute
		} else {
			info.Timeouts.Reservation = d
		}
	}

	if info.Timeouts.Lease == 0 {
		d, err := time.ParseDuration(r.DHCPConfig.LeaseTimeout)
		if err != nil {
			info.Timeouts.Lease = 6 * time.Hour
		} else {
			info.Timeouts.Lease = d
		}
	}

	if info.Timeouts.T2RenewalTime == 0 {
		d, err := time.ParseDuration(r.DHCPConfig.T2Timeout)
		if err != nil {
			info.Timeouts.T2RenewalTime = info.Timeouts.Lease / 2
		} else {
			info.Timeouts.T2RenewalTime = d
		}
	}

	if info.Timeouts.T1RenewalTime == 0 {
		d, err := time.ParseDuration(r.DHCPConfig.T1Timeout)
		if err != nil {
			info.Timeouts.T1RenewalTime = info.Timeouts.T2RenewalTime / 2
		} else {
			info.Timeouts.T1RenewalTime = d
		}
	}
}
