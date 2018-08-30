package resolver

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"log"
)

type Redis struct {
	Client *redis.Client
}

// REDIS STRUCTURE
// -------------------------------------
// key:						      					value:
// -------------------------------------
// v4;{mac};{ip}									{json}
// v4;{duid};{iaid};{ip}					{json}
// -------------------------------------

func (r Redis) AcknowledgeV4ByMAC(info *ClientInfoV4, mac, ip string) error {
	key := keyMAC(4, mac, ip)

	log.Printf("Receiving info about '%s' from cache.", key)

	result := r.Client.Get(key)
	if result.Err() != nil {
		log.Printf("Unable to receive info about '%s': %s", key, result.Err())
		return result.Err()
	}

	rawInfo, err := result.Bytes()
	if err != nil {
		log.Printf("Unable to extract info from '%s': %s", key, err)
		return err
	}

	err = json.Unmarshal(rawInfo, &info)
	if err != nil {
		log.Printf("Unable to reconstruct info from '%s': %s", key, err)
		return err
	}

	r.Client.Set(key, rawInfo, info.Timeouts.Lease)

	return nil
}

func (r Redis) AcknowledgeV4ByID(info *ClientInfoV4, duid, iaid, ip string) error {
	panic("implement me")
}

func (r Redis) ReserveV4ByMAC(info *ClientInfoV4, mac string) error {
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

func (r Redis) ReserveV4ByID(info *ClientInfoV4, duid, iaid string) error {
	panic("implement me")
}

func keyMAC(familiy uint8, mac, ip string) string {
	return fmt.Sprintf("v%d;%s;%s", familiy, mac, ip)
}
