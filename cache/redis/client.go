package redis

import (
	"fmt"
	"github.com/go-redis/redis"
)

// maybe wrap in struct in order to be able to hand out
// a new client when the config changes
func NewClient(config *RedisConfig) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       int(config.Database),
	})

	return client
}
