package main

import "testing"

// BenchmarkRouteSegment ...
func BenchmarkRouteSegment(b *testing.B) {
	config := ConsistentHashBalancerConfig{
		ReplicationFactor: 1000,
	}
	proxyEvents := make(chan *ProxyStatusEvent)
	lb := NewConsistentHashBalancer(config, proxyEvents)
	lb.addDomain("test.at.com")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = lb.RouteSegment("192.168.0.1", "test", "1.ts")
	}
}
