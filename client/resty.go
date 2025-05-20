package client

import (
	"bytes"
	"compress/gzip"
	"errors"
	"log"
	"os"

	utils "github.com/angelthump/atm3u8/utils"
	"github.com/bluenviron/gohlslib/pkg/playlist"
	"github.com/go-resty/resty/v2"
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

func GetM3u8(channel string, endURL string) ([]byte, error, int) {
	resp, _ := client.R().
		Get("http://" + utils.Config.Upstream + "/hls/" + channel + "/" + endURL)

	statusCode := resp.StatusCode()
	if statusCode != 200 {
		log.Printf("Unexpected status code, got %d instead", statusCode)
		return nil, errors.New(string(resp.Body())), statusCode
	}

	parsedM3u8, err := parseM3u8(resp.Body(), channel)
	if err != nil {
		return nil, err, statusCode
	}

	gzipM3u8, err := gZip(parsedM3u8)
	if err != nil {
		return nil, err, statusCode
	}

	return gzipM3u8, nil, statusCode
}

func parseM3u8(m3u8Bytes []byte, channel string) ([]byte, error) {
	pl, err := playlist.Unmarshal(m3u8Bytes)
	if err != nil {
		return nil, errors.New("Failed to decode m3u8..")
	}

	var newM3u8Bytes []byte

	switch pl := pl.(type) {
	case *playlist.Media:
		for _, seg := range pl.Segments {
			if seg == nil {
				continue
			}
			seg.URI = "https://" + hostname + "." + utils.Config.Domain + "/hls/" + channel + "/" + seg.URI
		}
		newM3u8Bytes, err = pl.Marshal()
		if err != nil {
			return nil, errors.New("Failed to Marshal m3u8..")
		}
	default:
		newM3u8Bytes = m3u8Bytes
	}

	return newM3u8Bytes, nil
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
