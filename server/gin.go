package server

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	client "github.com/angelthump/atm3u8/client"
	utils "github.com/angelthump/atm3u8/utils"
)

func Initalize() {
	if utils.Config.GinReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	router.SetTrustedProxies([]string{"127.0.0.1"})

	router.GET("/hls/:channel/index.m3u8", func(c *gin.Context) {
		channel := c.Param("channel")
		go func(channel string, ip_address string) {
			err := Rdb.ZAdd(Ctx, channel, &redis.Z{
				Score:  float64(time.Now().UnixMilli()),
				Member: ip_address,
			}).Err()
			if err != nil {
				log.Println(err)
				//log.Printf("Failed to ZAdd %s in %s", ip_address, channel)
			}
		}(channel, c.ClientIP())
		m3u8Bytes, err, statusCode := client.GetM3u8(channel)
		if err != nil {
			log.Printf("%v", err)
			if statusCode == 0 {
				c.String(http.StatusInternalServerError, "")
			} else {
				c.String(statusCode, "")
			}
			return
		}

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		c.Data(statusCode, "application/x-mpegURL", m3u8Bytes)
	})

	router.Run(":" + utils.Config.Port)
}
