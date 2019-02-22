package main

import "github.com/grafov/m3u8"

// LoadBalancer ...
type LoadBalancer interface {
	RouteSegment(sessionKey, channel, uri string) (string, error)
}

// RewriteMediaPlaylist ...
func RewriteMediaPlaylist(source *m3u8.MediaPlaylist, sessionKey, channel string, router LoadBalancer) (*m3u8.MediaPlaylist, error) {
	target := *source

	segments := source.Segments
	target.Segments = make([]*m3u8.MediaSegment, source.Count())

	for i := uint(0); i < source.Count(); i++ {
		segment := *segments[i]

		uri, err := router.RouteSegment(sessionKey, channel, segment.URI)
		if err != nil {
			return nil, err
		}

		segment.URI = uri
		target.Segments[i] = &segment
	}

	target.ResetCache()

	return &target, nil
}
