package main

import "sync/atomic"

// round-robin load balancer
func (p *Proxy) getNextServer() *Upstream {
	if len(p.servers) == 0 {
		return nil
	}

	i := atomic.AddUint64(&p.serverIndex, 1)

	return p.servers[i%uint64(len(p.servers))]
}
