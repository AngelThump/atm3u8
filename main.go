package main

import (
	"log"

	client "github.com/angelthump/atm3u8/client"
	server "github.com/angelthump/atm3u8/server"
	utils "github.com/angelthump/atm3u8/utils"
)

func main() {
	cfgPath, err := utils.ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	err = utils.NewConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	client.Initalize()
	server.Initalize()
}
