package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"time"
)

// health check for primary and replicas
// spawn goroutines for primary
// spawn goroutines for each replicas
func (p *Proxy) healthCheck() {
	go func() {
		if err := p.pingUpstream(p.session.UpPrimary); err != nil {
			return
		}
	}()

	for _, v := range p.session.Replicas {
		go func(replica *Upstream) {
			if err := p.pingUpstream(replica); err != nil {
				return
			}
		}(v)
	}
}

// for every one minute
// acquire the lock of each
// check the health of each
// release the lock of each
// if any of them is down, then mark the session as down/dead
func (p *Proxy) pingUpstream(upstream *Upstream) error {
	ticker := time.NewTicker(p.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case <-ticker.C:
			upstream.lock.Lock()

			healthy, lag, start := false, 0, time.Now()

			// Send ping request
			if upstream.Conn != nil {
				if err := checkUpstream(upstream); err != nil {
					p.logger.Warn().Err(err).Msgf("Ping failed for %s: %v", upstream.Addr, err)
				} else {
					lag = int(time.Since(start).Milliseconds())
					healthy = true
				}
			}

			// Update database
			_, err := p.sqliteDB.Exec(fmt.Sprintf("UPDATE upstreams SET healthy = %v, lag = %v WHERE addr = %v", healthy, lag, upstream.Addr))
			if err != nil {
				p.logger.Warn().Err(err).Msgf("Failed to update %s in database: %v", upstream.Addr, err)
			} else {
				p.logger.
					Info().
					Msgf("Pinged %s at %s: healthy=%v, lag=%dms",
						upstream.Addr, time.Now().Format("15:04:05"), healthy, lag)
			}

			upstream.Healthy = healthy
			upstream.Lag = lag

			upstream.lock.Unlock()
		}
	}
}

// Send a simple ping query
func checkUpstream(up *Upstream) error {
	_, err := up.Conn.Write(encodeSimpleQuery("SELECT 1"))
	if err != nil {
		log.Printf("Health check failed: write error to %s: %v", up.Addr, err)
		return err
	}

	if err = up.Conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Printf("Health check failed: set read deadline error to %s: %v", up.Addr, err)
		return err
	}

	buf := make([]byte, 512)

	if _, err = up.Conn.Read(buf); err != nil {
		log.Printf("Health check failed: read error from %s: %v", up.Addr, err)
		return err
	}

	return nil
}

func encodeSimpleQuery(sql string) []byte {
	payload := make([]byte, 5+len(sql)+1)
	payload[0] = 'Q'

	binary.BigEndian.PutUint32(payload[1:], uint32(len(sql)+5))

	copy(payload[5:], sql)
	payload[len(payload)-1] = 0

	return payload
}
