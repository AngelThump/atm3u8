package main

import (
	"fmt"
	"log"
	"sync/atomic"
)

// RoundRobinBalancerConfig ...
type RoundRobinBalancerConfig struct {
	ProxyServers []string `yaml:"proxyServers"`
}

// RoundRobinBalancer ...
type RoundRobinBalancer struct {
	config    RoundRobinBalancerConfig
	servers   []string
	nextIndex uint64
}

// NewRoundRobinBalancer ...
func NewRoundRobinBalancer(config RoundRobinBalancerConfig) *RoundRobinBalancer {
	log.Printf("created round robin load balancer with %d servers", len(config.ProxyServers))

	return &RoundRobinBalancer{
		servers: config.ProxyServers,
	}
}

// RouteSegment ...
func (r *RoundRobinBalancer) RouteSegment(channel, chunk string) (string, error) {
	index := atomic.AddUint64(&r.nextIndex, 1) % uint64(len(r.servers))
	return fmt.Sprintf("%s/hls/%s/%s", r.servers[index], channel, chunk), nil
}
