package main

import (
	"errors"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/spaolacci/murmur3"
)

// ErrNoServers ...
var ErrNoServers = errors.New("no servers found")

// ConsistentHashBalancerConfig ...
type ConsistentHashBalancerConfig struct {
	ReplicationFactor int `yaml:"replicationFactor"`
}

type consistentHashBalancerRingEntry struct {
	key    uint32
	domain *string
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
	config         ConsistentHashBalancerConfig
	hashRingLock   sync.RWMutex
	hashRingLength int
	hashRing       []consistentHashBalancerRingEntry
	domainSet      map[string]struct{}
}

// NewConsistentHashBalancer ...
func NewConsistentHashBalancer(config ConsistentHashBalancerConfig, proxyEvents chan *ProxyStatusEvent) *ConsistentHashBalancer {
	if config.ReplicationFactor == 0 {
		log.Fatal("replication factor is zero")
	}

	r := &ConsistentHashBalancer{
		config:    config,
		domainSet: map[string]struct{}{},
	}
	go r.handleProxyEvents(proxyEvents)

	return r
}

func (r *ConsistentHashBalancer) handleProxyEvents(proxyEvents chan *ProxyStatusEvent) {
	for event := range proxyEvents {
		switch event.Status {
		case ProxyStatusAdded:
			r.addDomain(event.Domain)
		case ProxyStatusRemoved:
			r.removeDomain(event.Domain)
		}
	}
}

func (r *ConsistentHashBalancer) addDomain(domain string) {
	if _, ok := r.domainSet[domain]; ok {
		return
	}
	r.domainSet[domain] = struct{}{}

	newHashRingLength := r.hashRingLength + r.config.ReplicationFactor
	hashRing := make([]consistentHashBalancerRingEntry, r.hashRingLength, newHashRingLength)
	r.hashRingLength = newHashRingLength

	r.hashRingLock.RLock()
	copy(hashRing, r.hashRing)
	r.hashRingLock.RUnlock()

	hash := murmur3.New64()
	hash.Write([]byte(domain))
	rand.Seed(int64(hash.Sum64() & math.MaxInt64))

	for i := 0; i < r.config.ReplicationFactor; i++ {
		ringEntry := consistentHashBalancerRingEntry{
			key:    uint32(rand.Int63n(math.MaxUint32)),
			domain: &domain,
		}
		hashRing = append(hashRing, ringEntry)
	}

	sort.Sort(consistentHashBalancerRing(hashRing))

	r.hashRingLock.Lock()
	r.hashRing = hashRing
	r.hashRingLock.Unlock()

	log.Printf("added %s to router", domain)
}

func (r *ConsistentHashBalancer) removeDomain(domain string) {
	if _, ok := r.domainSet[domain]; !ok {
		return
	}
	delete(r.domainSet, domain)

	hashRing := make([]consistentHashBalancerRingEntry, r.hashRingLength)
	r.hashRingLength -= r.config.ReplicationFactor

	r.hashRingLock.RLock()
	copy(hashRing, r.hashRing)
	r.hashRingLock.RUnlock()

	filteredHashRing := make([]consistentHashBalancerRingEntry, 0, r.hashRingLength)
	for _, entry := range hashRing {
		if *entry.domain != domain {
			filteredHashRing = append(filteredHashRing, entry)
		}
	}

	r.hashRingLock.Lock()
	r.hashRing = filteredHashRing
	r.hashRingLock.Unlock()

	log.Printf("removed %s from router", domain)
}

// RouteSegment ...
func (r *ConsistentHashBalancer) RouteSegment(sessionKey, channel, chunk string) (string, error) {
	hash := murmur3.New32()
	hash.Write([]byte(sessionKey))
	hash.Write([]byte(chunk))
	key := hash.Sum32()

	r.hashRingLock.RLock()
	defer r.hashRingLock.RUnlock()

	if len(r.hashRing) == 0 {
		return "", ErrNoServers
	}

	i := sort.Search(len(r.hashRing), func(i int) bool { return r.hashRing[i].key >= key })
	if i == len(r.hashRing) {
		i = 0
	}
	url := "https://" + *r.hashRing[i].domain + "/hls/" + channel + "/" + chunk

	return url, nil
}
