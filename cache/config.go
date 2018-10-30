package cache

import "github.com/cimnine/netbox-dhcp/cache/redis"

type CacheConfig struct {
	Type  string
	Redis redis.RedisConfig
}
