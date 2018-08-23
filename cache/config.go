package cache

import "github.com/ninech/nine-dhcp2/cache/redis"

type CacheConfig struct {
	Type  string
	Redis redis.RedisConfig
}
