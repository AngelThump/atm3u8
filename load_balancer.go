package main

// LoadBalancer ...
type LoadBalancer interface {
	RouteSegment(sessionKey, channel, uri string) (string, error)
}
