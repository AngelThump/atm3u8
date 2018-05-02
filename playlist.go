package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafov/m3u8"
)

// Playlist ...
type Playlist struct {
	channel      string
	lastModified int64
	duration     int64
	loaderLock   sync.Mutex
	loader       *PlaylistLoader
	valueLock    sync.RWMutex
	value        *m3u8.MediaPlaylist
}

// NewPlaylist ...
func NewPlaylist(channel string, loader *PlaylistLoader) *Playlist {
	return &Playlist{
		channel: channel,
		loader:  loader,
	}
}

// LastModified ...
func (p *Playlist) LastModified() time.Time {
	return time.Unix(0, atomic.LoadInt64(&p.lastModified))
}

// Age ...
func (p *Playlist) Age() time.Duration {
	return time.Since(p.LastModified())
}

// TargetDuration ...
func (p *Playlist) TargetDuration() time.Duration {
	return time.Duration(atomic.LoadInt64(&p.duration)) * time.Nanosecond
}

// Stale ...
func (p *Playlist) Stale() bool {
	return p.Age() >= p.TargetDuration()
}

// Load ...
func (p *Playlist) Load() error {
	if !p.Stale() {
		return nil
	}

	p.loaderLock.Lock()
	defer p.loaderLock.Unlock()

	if p.Stale() {
		atomic.StoreInt64(&p.lastModified, time.Now().UnixNano())

		playlist, err := p.loader.Get(p.channel)
		if err != nil {
			return fmt.Errorf("error loading playlist: %v", err)
		}

		p.valueLock.Lock()
		p.value = playlist
		p.valueLock.Unlock()

		durationNanos := playlist.TargetDuration * float64(time.Second/time.Nanosecond)
		atomic.StoreInt64(&p.duration, int64(durationNanos))
	}

	return nil
}

// Route ...
func (p *Playlist) Route(router LoadBalancer) (*m3u8.MediaPlaylist, error) {
	p.valueLock.RLock()
	value := p.value
	p.valueLock.RUnlock()

	playlist := *value
	playlist.Segments = make([]*m3u8.MediaSegment, int(value.Count()))

	for i := uint(0); i < value.Count(); i++ {
		segment := *value.Segments[i]

		uri, err := router.RouteSegment(p.channel, segment.URI)
		if err != nil {
			return nil, err
		}

		segment.URI = uri
		playlist.Segments[i] = &segment
	}

	playlist.ResetCache()

	return &playlist, nil
}
