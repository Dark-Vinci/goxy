package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

// chooseReplica picks a healthy replica from the list
// You could plug in different policies (round-robin, random, least-lag, etc.)
var rrCounter uint64 // shared round-robin counter

func chooseReplica(replicas []*Upstream) *Upstream {
	if len(replicas) == 0 {
		return nil
	}

	// Filter out unhealthy replicas
	healthy := make([]*Upstream, 0, len(replicas))
	for _, r := range replicas {
		if r.Healthy && r.Lag < 5000 { // example: max 5s lag
			healthy = append(healthy, r)
		}
	}

	if len(healthy) == 0 {
		// fallback: no healthy replicas, return first (caller can retry/fallback to primary)
		return replicas[0]
	}

	// Round-robin selection
	idx := atomic.AddUint64(&rrCounter, 1)
	return healthy[idx%uint64(len(healthy))]
}

func FromClient(clientConn, serverConn net.Conn, connID int64, wg *sync.WaitGroup) {
	defer wg.Done()
	reader, isStartup := bufio.NewReader(clientConn), true

	for {
		// Read data from client
		data := make([]byte, 16384) // Increased buffer size
		n, err := reader.Read(data)
		if err != nil {
			if err != io.EOF {
				log.Printf("FROM-CLIENT; [Conn %d] Error reading from client: %v", connID, err)
			}
			return
		}
		if n == 0 {
			continue
		}
		data = data[:n]

		// Log client data
		if isStartup && len(data) > 8 && data[4] == 0 && data[5] == 3 {
			params := parseStartupMessage(data[8:])
			log.Printf("FROM-CLIENT; [Conn %d] Client Startup: user=%s, database=%s, password=%v", connID, params["user"], params["database"], params)
			isStartup = false
		} else if data[0] == 'Q' && n > 5 { // here
			query := string(bytes.Trim(data[5:], "\x00"))
			log.Printf("FROM-CLIENT; [Conn %d] Client Query: %s", connID, query)

			// Filter queries by type
			checkAndLogQueryType(int(connID), query)

		} else if data[0] == 'P' && n > 5 {
			idx := bytes.IndexByte(data[5:], 0) + 5
			if idx < n-1 {
				query := string(bytes.Trim(data[idx+1:bytes.IndexByte(data[idx+1:], 0)+idx+1], "\x00"))
				log.Printf("FROM-CLIENT; [Conn %d] Client Prepared Statement: %s", connID, query)

				// Filter prepared statements too
				checkAndLogQueryType(int(connID), query)
			} else {
				log.Printf("FROM-CLIENT; [Conn %d] Client Parse: (malformed, %d bytes)", connID, n)
			}
		} else if data[0] == 'B' && n > 5 {
			params, err := parseBindParameters(data, n)
			if err != nil {
				log.Printf("FROM-CLIENT; [Conn %d] Client Bind: failed to parse parameters: %v", connID, err)
			} else {
				log.Printf("FROM-CLIENT; [Conn %d] Client Bind Parameters: %v", connID, params)
			}
		} else if data[0] == 'p' {
			log.Printf("FROM-CLIENT; [Conn %d] Client Authentication: (password or SASL data)", connID)
		} else if data[0] == 'D' && n > 5 {
			log.Printf("FROM-CLIENT; [Conn %d] Client Describe: %s", connID, parseDescribeMessage(data))
		} else if data[0] == 'E' && n == 5 {
			log.Printf("FROM-CLIENT; [Conn %d] Client Execute", connID)
		} else if data[0] == 'S' && n == 5 {
			log.Printf("FROM-CLIENT; [Conn %d] Client Sync", connID)
		} else if data[0] == 'X' && n == 5 {
			log.Printf("FROM-CLIENT; [Conn %d] Client Terminate", connID)
		} else {
			log.Printf("FROM-CLIENT; [Conn %d] Client -> PostgreSQL: %x", connID, data)
		}

		// Forward data to PostgreSQL
		_, err = serverConn.Write(data)
		if err != nil {
			log.Printf("[Conn %d] Error forwarding to PostgreSQL: %v", connID, err)
			return
		}
	}
}

func FromDB(serverConn, clientConn net.Conn, connID int64, wg *sync.WaitGroup) {
	reader := bufio.NewReader(serverConn)
	defer wg.Done()

	for {
		// Read message type (1 byte)
		msgType, err := reader.ReadByte()
		if err != nil {
			if err != io.EOF {
				log.Printf("FROM-POSTGRES; [Conn %d] Error reading message type from PostgreSQL: %v", connID, err)
			}
			return
		}

		// Read length (int32, includes length bytes but not the type byte)
		lengthBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, lengthBytes); err != nil {
			log.Printf("FROM-POSTGRES; [Conn %d] Error reading message length: %v", connID, err)
			return
		}
		msgLength := int(binary.BigEndian.Uint32(lengthBytes))
		if msgLength < 4 {
			log.Printf("FROM-POSTGRES; [Conn %d] Invalid message length: %d", connID, msgLength)
			return
		}

		// Read the rest of the message body
		body := make([]byte, msgLength-4)
		if _, err := io.ReadFull(reader, body); err != nil {
			log.Printf("FROM-POSTGRES; [Conn %d] Error reading message body: %v", connID, err)
			return
		}

		// Reconstruct the full message to forward to the client
		fullMsg := append([]byte{msgType}, append(lengthBytes, body...)...)

		// Log based on message type
		switch msgType {
		case 'R': // Authentication
			if len(body) >= 4 {
				authType := binary.BigEndian.Uint32(body[:4])
				switch authType {
				case 0:
					log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Authentication: Success", connID)
				case 10:
					sasl := string(bytes.Trim(body[4:], "\x00"))
					log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Authentication: SASL requested (%s)", connID, sasl)
				default:
					log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Authentication: Type %d", connID, authType)
				}
			}
		case 'C':
			tag := string(bytes.Trim(body, "\x00"))
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Command Complete: %s", connID, tag)
		case 'E':
			fields := parseErrorOrNotice(body)
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Error: %s", connID, fields["M"])
		case 'N':
			fields := parseErrorOrNotice(body)
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Notice: %s", connID, fields["M"])
		case 'T':
			columns := parseRowDescription(body)
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Row Description: %v", connID, columns)
		case 'D':
			values := parseDataRow(body)
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Data Row: %v", connID, values)
		case 'Z':
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Ready for Query", connID)
		case 'S':
			keyValue := parseParameterStatus(body)
			if len(keyValue) >= 2 {
				log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Parameter Status: %s=%s", connID, keyValue[0], keyValue[1])
			}
		case 'K':
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Backend Key Data", connID)
		case '1':
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Parse Complete", connID)
		case '2':
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL Bind Complete", connID)
		default:
			records, err := parsePostgresDataRow(body)
			if err != nil {
				log.Printf("FROM-POSTGRES; Error parsing hex records: %v", err)
			} else {
				log.Printf("FROM-POSTGRES; Records: %+v", records)
			}
			log.Printf("FROM-POSTGRES; [Conn %d] PostgreSQL -> Client: %x", connID, fullMsg)
		}

		// Forward the message to the client
		if _, err := clientConn.Write(fullMsg); err != nil {
			log.Printf("FROM-POSTGRES; [Conn %d] Error forwarding to client: %v", connID, err)
			return
		}
	}
}
