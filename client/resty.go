package client

import (
	"bytes"
	"compress/gzip"
	"errors"
	"log"
	"os"

	utils "github.com/angelthump/atm3u8/utils"
	"github.com/go-resty/resty/v2"
	"github.com/grafov/m3u8"
)

var hostname string
var client *resty.Client

func Initalize() {
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}

	client = resty.New()
}

func GetM3u8(channel string) ([]byte, error, int) {
	resp, _ := client.R().
		Get("http://" + utils.Config.Upstream + "/hls/" + channel + "/index.m3u8")

	statusCode := resp.StatusCode()
	if statusCode != 200 {
		log.Printf("Unexpected status code, got %d instead", statusCode)
		return nil, errors.New(string(resp.Body())), statusCode
	}

	buffer := bytes.NewBuffer(resp.Body())

	parsedM3u8, err := parseM3u8(*buffer, channel)
	if err != nil {
		return nil, err, statusCode
	}

	gzipM3u8, err := gZip(parsedM3u8)
	if err != nil {
		return nil, err, statusCode
	}

	return gzipM3u8, nil, statusCode
}

func parseM3u8(buffer bytes.Buffer, channel string) ([]byte, error) {
	p, _, err := m3u8.Decode(buffer, true)
	if err != nil {
		return nil, errors.New("Failed to decode m3u8..")
	}
	for _, segment := range p.(*m3u8.MediaPlaylist).Segments {
		if segment == nil {
			continue
		}
		segment.URI = "https://" + hostname + "." + utils.Config.Domain + "/hls/" + channel + "/" + segment.URI
	}
	return p.Encode().Bytes(), nil
}

func gZip(data []byte) (compressedData []byte, err error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)

	_, err = gz.Write(data)
	if err != nil {
		return nil, errors.New("Failed to gzip write m3u8..")
	}

	if err = gz.Flush(); err != nil {
		return nil, errors.New("Failed to gzip write m3u8..")
	}

	if err = gz.Close(); err != nil {
		return nil, errors.New("Failed to gzip write m3u8..")
	}

	return b.Bytes(), nil
}
