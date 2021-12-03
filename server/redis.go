package server

import (
	"context"

	utils "github.com/angelthump/atm3u8/utils"
	"github.com/go-redis/redis/v8"
)

var Rdb *redis.Client
var Ctx = context.Background()

func InitalizeRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     utils.Config.Redis.Hostname,
		Password: utils.Config.Redis.Password,
		DB:       0,
	})
}
