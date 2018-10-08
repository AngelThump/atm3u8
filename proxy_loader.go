package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// LivenessCheckConfig ...
type LivenessCheckConfig struct {
	IntervalSeconds  time.Duration `yaml:"intervalSeconds"`
	URLFormat        string        `yaml:"urlFormat"`
	SuccessThreshold int64         `yaml:"successThreshold"`
	FailureThreshold int64         `yaml:"failureThreshold"`
}

// ProxyLoaderConfig ...
type ProxyLoaderConfig struct {
	API                       string              `yaml:"api"`
	Region                    string              `yaml:"region"`
	DomainFormat              string              `yaml:"domainFormat"`
	APIRefreshIntervalSeconds time.Duration       `yaml:"apiRefreshIntervalSeconds"`
	LivenessCheck             LivenessCheckConfig `yaml:"livenessCheck"`
}

// ProxyStatus ...
type ProxyStatus int64

func (p ProxyStatus) String() string {
	switch p {
	case ProxyStatusAdded:
		return "Added"
	case ProxyStatusDown:
		return "Down"
	case ProxyStatusOK:
		return "OK"
	case ProxyStatusRemoved:
		return "Removed"
	}

	log.Fatal("invalid value for ProxyStatus")
	return ""
}

// proxy statuses
const (
	ProxyStatusAdded ProxyStatus = iota
	ProxyStatusDown
	ProxyStatusOK
	ProxyStatusRemoved
)

// ProxyStatusEvent ...
type ProxyStatusEvent struct {
	Domain string
	Status ProxyStatus
}

type proxyStatusNotifier struct {
	notifyChannelsLock sync.Mutex
	notifyChannels     []chan *ProxyStatusEvent
}

func (p *proxyStatusNotifier) Notify(ch chan *ProxyStatusEvent) {
	p.notifyChannelsLock.Lock()
	defer p.notifyChannelsLock.Unlock()

	p.notifyChannels = append(p.notifyChannels, ch)
}

func (p *proxyStatusNotifier) EmitNotification(domain string, status ProxyStatus) {
	p.notifyChannelsLock.Lock()
	defer p.notifyChannelsLock.Unlock()

	event := &ProxyStatusEvent{domain, status}
	for _, ch := range p.notifyChannels {
		ch <- event
	}
}

// ProxyLivenessChecker ...
type ProxyLivenessChecker struct {
	config            LivenessCheckConfig
	statusNotifier    *proxyStatusNotifier
	domain            string
	url               string
	ticker            *time.Ticker
	stop              chan struct{}
	statusLock        sync.Mutex
	status            ProxyStatus
	statusStableCount int64
	checkCount        int64
	lastEmittedStatus ProxyStatus
}

// NewProxyLivenessChecker ...
func NewProxyLivenessChecker(config LivenessCheckConfig, statusNotifier *proxyStatusNotifier, domain string) *ProxyLivenessChecker {
	return &ProxyLivenessChecker{
		config:         config,
		statusNotifier: statusNotifier,
		domain:         domain,
		url:            fmt.Sprintf(config.URLFormat, domain),
		stop:           make(chan struct{}, 1),
	}
}

func (p *ProxyLivenessChecker) updateStatus(status ProxyStatus) {
	p.statusLock.Lock()
	defer p.statusLock.Unlock()

	p.checkCount++

	if status == p.status {
		p.statusStableCount++
		return
	}

	p.status = status
	p.statusStableCount = 1
}

// Status ...
func (p *ProxyLivenessChecker) Status() ProxyStatus {
	p.statusLock.Lock()
	defer p.statusLock.Unlock()

	if p.status == ProxyStatusOK {
		if p.checkCount < p.config.SuccessThreshold || p.statusStableCount >= p.config.SuccessThreshold {
			return ProxyStatusOK
		}

		return ProxyStatusDown
	}

	if p.checkCount < p.config.FailureThreshold || p.statusStableCount >= p.config.FailureThreshold {
		return ProxyStatusDown
	}

	return ProxyStatusOK
}

// Start ...
func (p *ProxyLivenessChecker) Start() {
	if p.ticker != nil {
		log.Fatal("liveness checker already started")
	}
	p.ticker = time.NewTicker(p.config.IntervalSeconds * time.Second)

	for {
		_, err := http.Head(p.url)
		if err != nil {
			p.updateStatus(ProxyStatusDown)
		} else {
			p.updateStatus(ProxyStatusOK)
		}

		status := p.Status()
		if status != p.lastEmittedStatus {
			log.Printf("proxy status for %s changed from %s to %s",
				p.domain,
				p.lastEmittedStatus.String(),
				status.String(),
			)

			p.lastEmittedStatus = status
			p.statusNotifier.EmitNotification(p.domain, status)
		}

		select {
		case <-p.ticker.C:
		case <-p.stop:
			break
		}
	}
}

// Stop ...
func (p *ProxyLivenessChecker) Stop() {
	p.ticker.Stop()
	p.stop <- struct{}{}
}

type proxy struct {
	Domain          string
	Added           bool
	LivenessChecker *ProxyLivenessChecker
}

// ProxyLoader ...
type ProxyLoader struct {
	proxyStatusNotifier
	config  ProxyLoaderConfig
	ticker  *time.Ticker
	stop    chan struct{}
	proxies map[string]*proxy
}

// NewProxyLoader ...
func NewProxyLoader(config ProxyLoaderConfig) *ProxyLoader {
	return &ProxyLoader{
		config:  config,
		stop:    make(chan struct{}, 1),
		proxies: map[string]*proxy{},
	}
}

// Start ...
func (p *ProxyLoader) Start() {
	if p.ticker != nil {
		log.Fatal("proxy loader already started")
	}
	p.ticker = time.NewTicker(p.config.APIRefreshIntervalSeconds * time.Second)

	for {
		subdomains, err := p.loadSubdomains()
		if err == nil {
			p.updateProxies(subdomains)
		}

		select {
		case <-p.ticker.C:
		case <-p.stop:
			break
		}
	}
}

// Stop ...
func (p *ProxyLoader) Stop() {
	p.ticker.Stop()
	p.stop <- struct{}{}
}

func (p *ProxyLoader) loadSubdomains() ([]string, error) {
	res, err := http.Get(p.config.API)
	if err != nil {
		return nil, fmt.Errorf("http error: %v", err)
	}
	defer res.Body.Close()

	body := struct {
		Regions map[string][]string
	}{}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}

	subdomains, ok := body.Regions[p.config.Region]
	if !ok || len(subdomains) == 0 {
		return nil, fmt.Errorf("no subdomains found in region %s", p.config.Region)
	}

	return subdomains, nil
}

func (p *ProxyLoader) updateProxies(subdomains []string) {
	addedSubdomains := []string{}
	subdomainSet := map[string]struct{}{}
	for _, subdomain := range subdomains {
		if _, ok := p.proxies[subdomain]; !ok {
			addedSubdomains = append(addedSubdomains, subdomain)
			log.Printf("discovered added proxy %s", subdomain)
		}

		subdomainSet[subdomain] = struct{}{}
	}

	removed := []string{}
	for subdomain := range p.proxies {
		if _, ok := subdomainSet[subdomain]; !ok {
			removed = append(removed, subdomain)
			log.Printf("discovered removed proxy %s", subdomain)
		}
	}

	for _, subdomain := range removed {
		proxy := p.proxies[subdomain]

		p.EmitNotification(proxy.Domain, ProxyStatusRemoved)

		proxy.LivenessChecker.Stop()
		delete(p.proxies, subdomain)
	}

	for _, subdomain := range addedSubdomains {
		domain := fmt.Sprintf(p.config.DomainFormat, subdomain)

		livenessChecker := NewProxyLivenessChecker(p.config.LivenessCheck, &p.proxyStatusNotifier, domain)
		go livenessChecker.Start()

		proxy := &proxy{
			Domain:          domain,
			LivenessChecker: livenessChecker,
		}
		p.proxies[subdomain] = proxy

		p.EmitNotification(proxy.Domain, ProxyStatusAdded)
	}
}
