package main

import (
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net"
	"time"
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
			isHealthy := upstream.Healthy
			upstream.lock.Lock()
			//defer upstream.lock.Unlock()

			healthy, lag, start := false, 0, time.Now()

			// Send ping request
			if upstream.Conn != nil {
				if err := checkUpstream(upstream); err != nil {
					// destroy the connection and mark it as unhealthy
					upstream.Conn = nil
					p.logger.Warn().Err(err).Msgf("Ping failed for %s: %v, ID: %v", upstream.Addr, err, upstream.ID)
				} else {
					lag = int(time.Since(start).Milliseconds())
					healthy = true
				}
			} else {
				conn, err := net.Dial("tcp", upstream.Addr)
				if err == nil {
					healthy = true
					upstream.Conn = conn
				} else {
					p.logger.Fatal().Err(err).Msgf("Failed to re-connect to replica %v: %v", upstream.Addr, err)
				}

				lag = int(time.Since(start).Milliseconds())
			}

			// insert database
			_, err := p.sqliteDB.Exec(
				"INSERT INTO upstream_cron(id, healthy, lag, address, state_change, nth) VALUES (?, ?, ?, ?, ?, ?)",
				uuid.New(),
				boolToInt(healthy), // store as 1/0
				lag,
				upstream.Addr,
				boolToInt(isHealthy == healthy),
				p.nthCheck,
			)

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

			// HEALTH STATUS HAS CHANGED
			if isHealthy != healthy {
				fmt.Println("Upstream, healthy, isHealthy", healthy, isHealthy, upstream.ID)
				// WE NEED TO LOCK BEFORE WE ACCESS AND MODIFY THE UPSTREAMS
				p.lock.Lock()

				if healthy {
					// ADD TO HEALTHY, REMOVE FROM UNHEALTHY
					p.servers = append(p.servers, upstream)

					// REMOVE FROM UNHEALTH
					for i, v := range p.unhealthy {
						if v.ID == upstream.ID {
							p.unhealthy = append(p.unhealthy[:i], p.unhealthy[i+1:]...)
							break
						}
					}
				} else {
					fmt.Println("Unpacking healthy", upstream.ID, p.servers)
					// REMOVE FROM HEALTHy, ADD TO UNHEALTHy
					for i, v := range p.servers {
						if v.ID == upstream.ID {
							p.servers = append(p.servers[:i], p.servers[i+1:]...)
						}
						break
					}

					// ADD TO UNHEALTH
					p.unhealthy = append(p.unhealthy, upstream)
				}

				p.lock.Unlock()
				//upstream.lock.Unlock()
			}

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

	if err = up.Conn.SetReadDeadline(time.Now().Add(2 * time.Millisecond)); err != nil {
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
