package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/grafov/m3u8"
)

// PlaylistLoader ...
type PlaylistLoader struct {
	UpstreamServers []string
	CacheTTL        time.Duration
}

func (p *PlaylistLoader) get(url string, expectedPlaylistType m3u8.ListType) (m3u8.Playlist, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http error: %v", err)
	}
	defer res.Body.Close()

	playlist, playlistType, err := m3u8.DecodeFrom(res.Body, false)
	if err != nil {
		return nil, fmt.Errorf("malformed playlist: %v", err)
	}
	if playlistType != expectedPlaylistType {
		return nil, errors.New("invalid playlist format")
	}

	return playlist, nil
}

// Get ...
func (p *PlaylistLoader) Get(channel string, playlistType m3u8.ListType) (m3u8.Playlist, error) {
	var urlFormat string
	switch playlistType {
	case m3u8.MASTER:
		urlFormat = fmt.Sprintf("%%s/hls/%s.m3u8", channel)
	case m3u8.MEDIA:
		urlFormat = fmt.Sprintf("%%s/hls/%s/index.m3u8", channel)
	}

	for _, server := range p.UpstreamServers {
		playlist, err := p.get(fmt.Sprintf(urlFormat, server), playlistType)
		if err == nil {
			return playlist, nil
		}
		log.Printf("error loading playlist: %v", err)
	}
	return nil, errors.New("error loading playlist from upstream servers")
}
