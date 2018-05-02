package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/grafov/m3u8"
)

// PlaylistLoader ...
type PlaylistLoader struct {
	UpstreamServers []string
}

func (p *PlaylistLoader) get(url string) (*m3u8.MediaPlaylist, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http error: %v", err)
	}
	defer res.Body.Close()

	playlist, playlistType, err := m3u8.DecodeFrom(res.Body, false)
	if err != nil {
		return nil, fmt.Errorf("malformed playlist: %v", err)
	}
	if playlistType != m3u8.MEDIA {
		return nil, errors.New("invalid playlist format")
	}

	return playlist.(*m3u8.MediaPlaylist), nil
}

func (p *PlaylistLoader) formatURL(server, channel string) string {
	return fmt.Sprintf("%s/hls/%s/index.m3u8", server, channel)
}

// Get ...
func (p *PlaylistLoader) Get(channel string) (*m3u8.MediaPlaylist, error) {
	for _, server := range p.UpstreamServers {
		playlist, err := p.get(p.formatURL(server, channel))
		if err == nil {
			return playlist, nil
		}
		log.Printf("error loading playlist: %v", err)
	}
	return nil, errors.New("error loading playlist from upstream servers")
}
