package explorer

import (
	"errors"
	"github.com/beego/beego/v2/core/logs"
	goredis "github.com/go-redis/redis"
	"poly-bridge/conf"
	"strconv"
	"time"
)

const (
	_crossTxCounter = "CrossTxCounter"
)

type RedisCache struct {
	c      *goredis.Client
	config *conf.RedisConfig
}

func GetRedisClient(redisConfig *conf.RedisConfig) (*RedisCache, error) {
	if redisConfig.DialTimeout <= 0 || redisConfig.ReadTimeout <= 0 || redisConfig.WriteTimeout <= 0 {
		//panic("must config redis timeout")
		logs.Error("must config redis timeout")
		return &RedisCache{
			c:      &goredis.Client{},
			config: redisConfig,
		}, errors.New("must config redis timeout")
	}
	options := &goredis.Options{
		Network:      redisConfig.Proto,
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DialTimeout:  redisConfig.DialTimeout * time.Second,
		ReadTimeout:  redisConfig.ReadTimeout * time.Second,
		WriteTimeout: redisConfig.WriteTimeout * time.Second,
		PoolSize:     redisConfig.PoolSize,
		IdleTimeout:  redisConfig.IdleTimeout * time.Second,
	}
	redisCache := &RedisCache{
		c:      goredis.NewClient(options),
		config: redisConfig,
	}
	return redisCache, nil
}

func (r *RedisCache) SetCrossTxCounter(counter int64) (err error) {
	key := _crossTxCounter
	if _, err = r.c.Set(key, counter, r.config.Expiration*time.Second).Result(); err != nil {
		err = errors.New(err.Error() + "add SetCrossTxCounter")
	}
	return
}
func (r *RedisCache) GetCrossTxCounter() (counter int64, err error) {
	key := _crossTxCounter
	resp, err := r.c.Get(key).Result()
	if err != nil {
		err = errors.New(err.Error() + "cache GetCrossTxCounter")
		return
	}
	count, err := strconv.Atoi(resp)
	counter = int64(count)
	if err != nil {
		err = errors.New(err.Error() + "cache GetCrossTxCounter Atoi")
	}
	return
}
