package main

import "sync"

// PlaylistService ...
type PlaylistService struct {
	loader     *PlaylistLoader
	valuesLock sync.RWMutex
	values     map[string]*Playlist
}

// NewPlaylistService ...
func NewPlaylistService(loader *PlaylistLoader) *PlaylistService {
	return &PlaylistService{
		loader: loader,
		values: make(map[string]*Playlist),
	}
}

// Load ...
func (p *PlaylistService) Load(channel string) (*Playlist, error) {
	p.valuesLock.RLock()
	value, ok := p.values[channel]
	p.valuesLock.RUnlock()

	if !ok {
		p.valuesLock.Lock()
		value, ok = p.values[channel]
		if !ok {
			value = NewPlaylist(channel, p.loader)
			p.values[channel] = value
		}
		p.valuesLock.Unlock()
	}

	if err := value.Load(); err != nil {
		return nil, err
	}

	return value, nil
}
