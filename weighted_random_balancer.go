package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
)

// WeightedRandomBalancerEntry ...
type WeightedRandomBalancerEntry struct {
	Address string  `yml:"address"`
	Weight  float64 `yml:"weight"`
}

// WeightedRandomBalancerConfig ...
type WeightedRandomBalancerConfig struct {
	ProxyServers []WeightedRandomBalancerEntry `yaml:"proxyServers"`
}

// WeightedRandomBalancer ...
type WeightedRandomBalancer struct {
	entries []WeightedRandomBalancerEntry
	sum     float64
}

// NewWeightedRandomBalancer ...
func NewWeightedRandomBalancer(config WeightedRandomBalancerConfig) *WeightedRandomBalancer {
	var sum float64
	for _, entry := range config.ProxyServers {
		sum += entry.Weight
	}
	if sum == 0 {
		log.Fatal("all servers have zero weight")
	}

	log.Printf("created weighted random load balancer with %d servers", len(config.ProxyServers))
	return &WeightedRandomBalancer{
		entries: config.ProxyServers,
		sum:     sum,
	}
}

// RouteSegment ...
func (r *WeightedRandomBalancer) RouteSegment(channel, chunk string) (string, error) {
	v := rand.Float64() * r.sum

	for _, entry := range r.entries {
		if v < entry.Weight {
			return fmt.Sprintf("%s/hls/%s/%s", entry.Address, channel, chunk), nil
		}
		v -= entry.Weight
	}

	return "", errors.New("error selecting server")
}
