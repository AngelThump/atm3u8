package main

// LoadBalancer ...
type LoadBalancer interface {
	RouteSegment(channel, uri string) (string, error)
}
