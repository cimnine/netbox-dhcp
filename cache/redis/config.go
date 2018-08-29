package redis

type RedisConfig struct {
	Host     string
	Port     uint16
	Password string
	Database uint8
}
