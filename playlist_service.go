package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafov/m3u8"
)

// CacheTTL playlist cache entry ttl
const CacheTTL = time.Second

// PlaylistService ...
type PlaylistService struct {
	loader    *PlaylistLoader
	cacheLock sync.RWMutex
	cache     map[playlistCacheKey]*playlistCacheEntry
}

// NewPlaylistService ...
func NewPlaylistService(loader *PlaylistLoader) *PlaylistService {
	p := &PlaylistService{
		loader: loader,
		cache:  make(map[playlistCacheKey]*playlistCacheEntry),
	}

	return p
}

// Load ...
func (p *PlaylistService) Load(channel string, listType m3u8.ListType) (m3u8.Playlist, error) {
	key := playlistCacheKey{channel, listType}

	p.cacheLock.RLock()
	cacheEntry, ok := p.cache[key]
	p.cacheLock.RUnlock()

	if !ok {
		p.cacheLock.Lock()
		cacheEntry, ok = p.cache[key]
		if !ok {
			cacheEntry = newPlaylistCacheEntry(channel, listType, p.loader)
			p.cache[key] = cacheEntry
		}
		p.cacheLock.Unlock()
	}

	if err := cacheEntry.Load(); err != nil {
		log.Printf("error loading channel %s: %v", channel, err)
		return nil, err
	}

	return cacheEntry.Value(), nil
}

type playlistCacheKey struct {
	Channel  string
	listType m3u8.ListType
}

type playlistCacheEntry struct {
	channel      string
	listType     m3u8.ListType
	lastModified int64
	loaderLock   sync.Mutex
	loader       *PlaylistLoader
	valueLock    sync.RWMutex
	value        m3u8.Playlist
}

func newPlaylistCacheEntry(channel string, listType m3u8.ListType, loader *PlaylistLoader) *playlistCacheEntry {
	return &playlistCacheEntry{
		channel:  channel,
		listType: listType,
		loader:   loader,
	}
}

func (p *playlistCacheEntry) LastModified() time.Time {
	return time.Unix(0, atomic.LoadInt64(&p.lastModified))
}

func (p *playlistCacheEntry) Age() time.Duration {
	return time.Since(p.LastModified())
}

func (p *playlistCacheEntry) Stale() bool {
	return p.Age() >= CacheTTL
}

func (p *playlistCacheEntry) Load() error {
	if !p.Stale() {
		return nil
	}

	p.loaderLock.Lock()
	defer p.loaderLock.Unlock()

	if p.Stale() {
		requestTime := time.Now().UnixNano()

		playlist, err := p.loader.Get(p.channel, p.listType)
		if err != nil {
			return fmt.Errorf("error loading playlist: %v", err)
		}

		p.valueLock.Lock()
		p.value = playlist
		p.valueLock.Unlock()

		atomic.StoreInt64(&p.lastModified, requestTime)
	}

	return nil
}

func (p *playlistCacheEntry) Value() m3u8.Playlist {
	p.valueLock.Lock()
	defer p.valueLock.Unlock()
	return p.value
}
