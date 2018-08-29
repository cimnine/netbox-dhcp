package resolver

import "github.com/go-redis/redis"

type Redis struct {
	Client *redis.Client
}

func (r Redis) AcknowledgeV4ByMAC(mac, ip string) (ClientInfoV4, error) {
	panic("implement me")
}

func (r Redis) AcknowledgeV4ByID(duid, iaid, ip string) (ClientInfoV4, error) {
	panic("implement me")
}

func (r Redis) ReserveV4ByMAC(mac string, info ClientInfoV4) error {
	// TODO
	return nil
}

func (r Redis) ReserveV4ByID(duid, iaid string, info ClientInfoV4) error {
	// TODO
	return nil
}
