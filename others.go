package main

import (
	"net"
	"strings"
)

type Upstream struct {
	Addr    string
	Role    UpstreamRole
	Healthy bool
	Lag     int
	Pool    *ConnPool // your pool wrapper over net.Conn
}

type Session struct {
	ClientConn   net.Conn
	UpPrimary    *Upstream
	Replicas     []*Upstream
	InTxn        bool
	Pinned       *Upstream             // set when InTxn
	StmtClass    map[string]QueryClass // prepared-name -> class
	UnnamedClass *QueryClass
}

func handleClientQuery(s *Session, query string) *Upstream {
	// classify query
	cls := classifySQL(query) // returns QueryRead or QueryWrite

	// decide where to send
	upstream := decideDestination(s, cls)

	if upstream == nil {
		//log.Printf("[Conn %d] No upstream available for query: %s", s.ID, query)
		// optionally send error back to client
		return nil
	}

	return upstream

	// forward query to chosen upstream
	//forwardQueryToUpstream(upstream, query, s)
}

func decideDestination(s *Session, cls QueryClass) *Upstream {
	if s.InTxn {
		if s.Pinned == nil {
			// pin on first statement of txn; if write => primary; if read you may choose replica but safer to pin primary
			if cls == QueryRead {
				// choose replica or primary depending on your policy; safer = primary
				s.Pinned = s.UpPrimary
			} else {
				s.Pinned = s.UpPrimary
			}
		}

		return s.Pinned
	}

	if cls == QueryRead {
		return chooseReplica(s.Replicas) // skip unhealthy/high-lag ones
	}

	return s.UpPrimary
}

func classifySQL(sql string) QueryClass {
	var (
		q  = strings.TrimSpace(sql)
		uq = strings.ToUpper(q)
	)

	if strings.HasPrefix(uq, "SELECT") {
		if strings.Contains(uq, " FOR UPDATE") || strings.Contains(uq, " FOR SHARE") {
			return QueryWrite // lock semantics require primary
		}

		// EXPLAIN ANALYZE actually runs the query; keep it primary by marking write
		if strings.HasPrefix(uq, "EXPLAIN ANALYZE") {
			return QueryWrite
		}

		return QueryRead
	}

	if strings.HasPrefix(uq, "BEGIN") || strings.HasPrefix(uq, "START TRANSACTION") {
		return QueryUnknown // txn control; handle separately
	}

	if strings.HasPrefix(uq, "COMMIT") || strings.HasPrefix(uq, "ROLLBACK") || strings.HasPrefix(uq, "END") {
		return QueryUnknown
	}

	for _, p := range writePrefixes {
		if strings.HasPrefix(uq, p) {
			return QueryWrite
		}
	}

	// default conservative
	return QueryWrite
}
