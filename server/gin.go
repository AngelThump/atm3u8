package server

import (
	"log"
	"net/http"
	"regexp"
	"strings"
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

	router.GET("/hls/:channel/:endURL", func(c *gin.Context) {
		endURL := c.Param("endURL")

		if !strings.HasSuffix(endURL, ".m3u8") {
			c.AbortWithStatus(400)
			return
		}

		channel := c.Param("channel")
		m3u8Bytes, err, statusCode := client.GetM3u8(channel, endURL)
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

		//Only count as viewer if 200 status code..
		if statusCode == 200 {
			go func(channel string, ip_address string) {
				regex, _ := regexp.Compile("_.*$")
				channel = regex.ReplaceAllString(channel, "")
				err := Rdb.ZAdd(Ctx, channel, &redis.Z{
					Score:  float64(time.Now().UnixMilli()),
					Member: ip_address,
				}).Err()
				if err != nil {
					log.Println(err)
				}
			}(channel, c.ClientIP())
		}
	})

	router.Run(":" + utils.Config.Port)
}
