package main

import (
	"regexp"
	"strings"
)

// initializePatterns sets up regex patterns for query classification
func (p *Proxy) initializePatterns() {
	writePatterns := []string{
		`^\s*(INSERT|UPDATE|DELETE|CREATE|DROP|ALTER|TRUNCATE|GRANT|REVOKE)\s+`,
		`^\s*(BEGIN|START\s+TRANSACTION|COMMIT|ROLLBACK)\s*`,
		`^\s*(COPY\s+\w+\s+FROM)\s+`,
		`^\s*(CALL|DO)\s+`,
		`^\s*(LOCK|UNLOCK)\s+`,
		`^\s*SET\s+((?!.*session_replication_role\s*=\s*replica).)*$`,
	}

	readPatterns := []string{
		`^\s*(SELECT|WITH|EXPLAIN|ANALYZE)\s+`,
		`^\s*(SHOW|DESCRIBE|DESC)\s+`,
		`^\s*(COPY\s+\w+\s+TO)\s+`,
		`^\s*SET\s+session_replication_role\s*=\s*replica`,
	}

	for _, pattern := range writePatterns {
		if regex, err := regexp.Compile("(?i)" + pattern); err == nil {
			p.writePatterns = append(p.writePatterns, regex)
		}
	}

	for _, pattern := range readPatterns {
		if regex, err := regexp.Compile("(?i)" + pattern); err == nil {
			p.readPatterns = append(p.readPatterns, regex)
		}
	}
}

// classifyQuery determines if a query is read or write operation
func (p *Proxy) classifyQuery(query string) QueryClass {
	trimmedQuery := strings.TrimSpace(query)

	for _, regex := range p.readPatterns {
		if regex.MatchString(trimmedQuery) {
			return QueryRead
		}
	}

	for _, regex := range p.writePatterns {
		if regex.MatchString(trimmedQuery) {
			return QueryWrite
		}
	}

	return QueryWrite // Default to write for safety
}
