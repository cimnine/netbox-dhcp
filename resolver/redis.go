package resolver

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cimnine/netbox-dhcp/dhcp/v4"
	"github.com/go-redis/redis"
)

type Redis struct {
	Client *redis.Client
}

// REDIS STRUCTURE
// --------------------------------------------------
// key:						      					value:	timeout:
// --------------------------------------------------
// v4;offer;{xid}     						{json}  reservation
// v4;lease;{mac}     						{json}  lease
// v4;lease;{duid};{iaid}     		{json}  lease
// --------------------------------------------------

func (r Redis) AcknowledgeV4ByMAC(info *v4.ClientInfoV4, xid, mac, ip string) error {
	return r.acknowledgeV4(info, xid, keyMAC(4, mac))
}

func (r Redis) AcknowledgeV4ByID(info *v4.ClientInfoV4, xid, duid, iaid, ip string) error {
	return r.acknowledgeV4(info, xid, keyClientID(4, duid, iaid))
}

func (r Redis) ReserveV4(info *v4.ClientInfoV4, xid string) error {
	return r.storeOffer(info, xid)
}

func (r Redis) ReleaseV4ByMAC(xid, mac, ip string) error {
	return r.removeLease(keyMAC(4, mac))
}

func (r Redis) ReleaseV4ByID(xid, duid, iaid, ip string) error {
	return r.removeLease(keyClientID(4, duid, iaid))
}

func (r Redis) acknowledgeV4(info *v4.ClientInfoV4, xid, leaseKey string) error {
	keyXID := keyXID(4, xid)

	if result := r.Client.Get(keyXID); result.Err() == nil {
		log.Printf("Persisting the offer with transaction id '%s'", xid)
		return r.persistOffer(info, xid, leaseKey)
	}

	log.Printf("No offer for transaction '%s' found. Now looking for lease '%s'.", xid, leaseKey)
	return r.extendLease(info, xid, leaseKey)
}

func (r Redis) persistOffer(info *v4.ClientInfoV4, xid, leaseKey string) error {
	keyXID := keyXID(4, xid)

	err := r.extendLease(info, xid, keyXID)
	if err != nil {
		log.Printf("Unable to extend the offer for transaction '%s' and turning it into a lease.", xid)
		return err
	}

	renameResult := r.Client.Rename(keyXID, leaseKey)
	if renameResult.Err() != nil {
		log.Printf("Unable to rename '%s' to '%s': %s", keyXID, leaseKey, renameResult.Err())
		return renameResult.Err()
	}

	log.Printf("Persisted '%s' as '%s' and reset TTL", keyXID, leaseKey)

	return nil
}

func (r Redis) extendLease(info *v4.ClientInfoV4, _, leaseKey string) error {
	log.Printf("Receiving info about '%s' from cache.", leaseKey)

	result := r.Client.Get(leaseKey)
	if result.Err() != nil {
		log.Printf("Unable to receive info about '%s': %s", leaseKey, result.Err())
		return result.Err()
	}

	rawInfo, err := result.Bytes()
	if err != nil {
		log.Printf("Unable to extract info from '%s': %s", leaseKey, err)
		return err
	}

	err = json.Unmarshal(rawInfo, info)
	if err != nil {
		log.Printf("Unable to reconstruct info from '%s': %s", leaseKey, err)
		return err
	}

	expireResult := r.Client.Expire(leaseKey, info.Timeouts.Lease)
	if expireResult.Err() != nil {
		log.Printf("Unable to extend TTL on '%s': %s", leaseKey, expireResult.Err())
		return expireResult.Err()
	}

	return nil
}

func (r Redis) removeLease(key string) error {
	log.Printf("Releasing '%s' from cache.", key)

	result := r.Client.Del(key)
	if result.Err() != nil {
		log.Printf("Error while releasing '%s' from cache.", key)
		return result.Err()
	} else if val := result.Val(); val > 1 {
		log.Printf("%d KEYS WERE REMOVED! THIS SHOULD NOT HAPPEN! Key was '%s'.", val, key)
		return fmt.Errorf("%d keys removed for '%s' instead of 0 or 1", val, key)
	}

	log.Printf("Released %d keys (0 and 1 are ok).", result.Val())
	return nil
}

func (r Redis) storeOffer(info *v4.ClientInfoV4, xid string) error {
	infoAsJson, err := json.Marshal(info)
	if err != nil {
		log.Printf("Can't convert payload for transaction '%s': %s", xid, err)
		return err
	}

	key := keyXID(4, xid)

	log.Printf("Writing info about '%s' to the cache.", key)

	status := r.Client.Set(key, infoAsJson, info.Timeouts.Reservation)
	if status.Err() != nil {
		log.Printf("Can't add info for '%s' to the cache: %s", key, status.Err())
		return status.Err()
	}

	log.Printf("Wrote info about '%s' to the cache.", key)

	return nil
}

func keyXID(family uint8, xid string) string {
	return fmt.Sprintf("v%d;%s", family, xid)
}

func keyMAC(family uint8, mac string) string {
	return fmt.Sprintf("v%d;%s", family, strings.ToUpper(mac))
}

func keyClientID(family uint8, duid, iaid string) string {
	return fmt.Sprintf("v%d;%s;%s", family, duid, iaid)
}
