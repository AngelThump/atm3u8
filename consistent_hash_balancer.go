package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"

	"github.com/spaolacci/murmur3"
)

// ConsistentHashBalancerEntry ...
type ConsistentHashBalancerEntry struct {
	Address string  `yml:"address"`
	Weight  float64 `yml:"weight"`
}

// ConsistentHashBalancerConfig ...
type ConsistentHashBalancerConfig struct {
	ReplicationFactor int                            `yaml:"replicationFactor"`
	ProxyServers      []*ConsistentHashBalancerEntry `yaml:"proxyServers"`
}

type consistentHashBalancerRingEntry struct {
	key   uint32
	value *ConsistentHashBalancerEntry
}

type consistentHashBalancerRing []consistentHashBalancerRingEntry

func (c consistentHashBalancerRing) Len() int {
	return len(c)
}

func (c consistentHashBalancerRing) Less(i, j int) bool {
	return c[i].key < c[j].key
}

func (c consistentHashBalancerRing) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// ConsistentHashBalancer ...
type ConsistentHashBalancer struct {
	hashRing []consistentHashBalancerRingEntry
}

// NewConsistentHashBalancer ...
func NewConsistentHashBalancer(config ConsistentHashBalancerConfig) *ConsistentHashBalancer {
	var weightSum float64
	for _, entry := range config.ProxyServers {
		weightSum += entry.Weight
	}
	if weightSum == 0 {
		log.Fatal("all servers have zero weight")
	}

	if config.ReplicationFactor == 0 {
		log.Fatal("replication factor is zero")
	}

	c := &ConsistentHashBalancer{}
	ringSize := config.ReplicationFactor * len(config.ProxyServers)
	for _, entry := range config.ProxyServers {
		replicaCount := int((entry.Weight / weightSum) * float64(ringSize))
		for i := 0; i < replicaCount; i++ {
			ringEntry := consistentHashBalancerRingEntry{
				key:   uint32(rand.Int63n(math.MaxUint32)),
				value: entry,
			}
			c.hashRing = append(c.hashRing, ringEntry)
		}
	}

	sort.Sort(consistentHashBalancerRing(c.hashRing))

	log.Printf("created consistent hash load balancer with %d servers", len(config.ProxyServers))
	return c
}

// RouteSegment ...
func (r *ConsistentHashBalancer) RouteSegment(sessionKey, channel, chunk string) (string, error) {
	hash := murmur3.New32()
	hash.Write([]byte(sessionKey))
	hash.Write([]byte(chunk))
	key := hash.Sum32()

	i := sort.Search(len(r.hashRing), func(i int) bool { return r.hashRing[i].key > key })
	if i == len(r.hashRing) {
		i = 0
	}

	return fmt.Sprintf("%s/hls/%s/%s", r.hashRing[i].value.Address, channel, chunk), nil
}
