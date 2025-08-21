package main

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
)

// health check for primary and replicas
// spawn goroutines for primary
// spawn goroutines for each replicas
func (p *Proxy) healthCheck() {
	p.nthCheck = 1

	// for healthy servers
	for _, v := range p.servers {
		go func(replica *Upstream) {
			if err := p.pingUpstream(replica); err != nil {
				return
			}
		}(v)
	}

	// unhealth server pings
	for _, v := range p.unhealthy {
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
			// store previous health state
			prevHealthy := upstream.Healthy

			upstream.lock.Lock()
			healthy, lag, start := false, 0, time.Now()

			// Send ping request
			//if prevHealthy {
			if err := checkUpstream(upstream); err != nil {
				p.logger.Warn().Err(err).Msgf(
					"Ping failed for %s, ID: %v", upstream.Addr, upstream.ID,
				)
			} else {
				lag = int(time.Since(start).Milliseconds())
				healthy = true
			}

			// insert database
			_, err := p.sqliteDB.Exec(
				`INSERT INTO upstream_cron(id, healthy, lag, address, state_change, nth)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				uuid.New(),
				boolToInt(healthy), // store as 1/0
				lag,
				upstream.Addr,
				boolToInt(prevHealthy != healthy), // state change = true if changed
				p.nthCheck,
			)

			if err != nil {
				p.logger.Warn().Err(err).Msgf(
					"Failed to update %s in database", upstream.Addr,
				)
			} else {
				p.logger.Info().Msgf(
					"Pinged %s at %s: healthy=%v, lag=%dms",
					upstream.Addr, time.Now().Format("15:04:05"), healthy, lag,
				)
			}

			// update upstream state
			upstream.Healthy = healthy
			upstream.Lag = lag
			upstream.lock.Unlock()

			// HEALTH STATUS HAS CHANGED
			if prevHealthy != healthy {
				p.lock.Lock()
				if healthy {
					// move from unhealthy → healthy
					for i, v := range p.unhealthy {
						if v.ID == upstream.ID {
							p.unhealthy = append(p.unhealthy[:i], p.unhealthy[i+1:]...)
							break
						}
					}

					p.servers = append(p.servers, upstream)

				} else {
					// move from healthy → unhealthy
					for i, v := range p.servers {
						if v.ID == upstream.ID {
							p.servers = append(p.servers[:i], p.servers[i+1:]...)
							break
						}
					}

					p.unhealthy = append(p.unhealthy, upstream)
				}
				p.lock.Unlock()
			}
		}
	}
}

// Send a simple ping query
func checkUpstream(up *Upstream) error {
	conn, err := net.Dial("tcp", up.Addr)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", up.Addr, err)
		return err
	}

	_, err = conn.Write(encodeSimpleQuery("SELECT 1"))
	if err != nil {
		log.Printf("Health check failed: write error to %s: %v", up.Addr, err)
		return err
	}

	if err = conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Printf("Health check failed: set read deadline error to %s: %v", up.Addr, err)
		return err
	}

	buf := make([]byte, 512)

	if _, err = conn.Read(buf); err != nil {
		if !errors.Is(err, io.EOF) {
			log.Printf("Health check failed: read error from %s: %v", up.Addr, err)
			return err
		}
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
