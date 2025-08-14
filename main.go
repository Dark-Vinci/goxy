package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
)

// Config holds proxy configuration
type Config struct {
	listenAddr string
	pgAddr     string
}

// Proxy represents the PostgreSQL proxy
type Proxy struct {
	config      *Config
	connCounter uint64 // Atomic counter for connection IDs
}

// NewProxy creates a new Proxy instance
func NewProxy(config *Config) *Proxy {
	return &Proxy{config: config}
}

// Start runs the proxy server
func (p *Proxy) Start() error {
	listener, err := net.Listen("tcp", p.config.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", p.config.listenAddr, err)
	}
	defer listener.Close()

	log.Printf("Proxy listening on %s, forwarding to %s", p.config.listenAddr, p.config.pgAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		connID := atomic.AddUint64(&p.connCounter, 1)
		go p.handleConnection(clientConn, connID)
	}
}

// handleConnection manages a single client connection
func (p *Proxy) handleConnection(clientConn net.Conn, connID uint64) {
	defer clientConn.Close()

	// Connect to PostgreSQL server
	pgConn, err := net.Dial("tcp", p.config.pgAddr)
	if err != nil {
		log.Printf("[Conn %d] Failed to connect to PostgreSQL: %v", connID, err)
		return
	}
	defer pgConn.Close()

	log.Printf("[Conn %d] New client connection established", connID)

	var wg sync.WaitGroup
	wg.Add(2)

	// Client to PostgreSQL with logging
	//go func() {
	//	defer wg.Done()
	//	reader := bufio.NewReader(clientConn)
	//	isStartup := true
	//	for {
	//		// Read data from client
	//		data := make([]byte, 16384) // Increased buffer size
	//		n, err := reader.Read(data)
	//		if err != nil {
	//			if err != io.EOF {
	//				log.Printf("FROM-CLIENT; [Conn %d] Error reading from client: %v", connID, err)
	//			}
	//			return
	//		}
	//		if n == 0 {
	//			continue
	//		}
	//		data = data[:n]
	//
	//		// Log client data
	//		if isStartup && len(data) > 8 && data[4] == 0 && data[5] == 3 {
	//			params := parseStartupMessage(data[8:])
	//			log.Printf("FROM-CLIENT; [Conn %d] Client Startup: user=%s, database=%s, password=%v", connID, params["user"], params["database"], params)
	//			isStartup = false
	//		} else if data[0] == 'Q' && n > 5 {
	//			query := string(bytes.Trim(data[5:], "\x00"))
	//			log.Printf("FROM-CLIENT; [Conn %d] Client Query: %s", connID, query)
	//		} else if data[0] == 'P' && n > 5 {
	//			idx := bytes.IndexByte(data[5:], 0) + 5
	//			if idx < n-1 {
	//				query := string(bytes.Trim(data[idx+1:bytes.IndexByte(data[idx+1:], 0)+idx+1], "\x00"))
	//				log.Printf("FROM-CLIENT; [Conn %d] Client Prepared Statement: %s", connID, query)
	//			} else {
	//				log.Printf("FROM-CLIENT; [Conn %d] Client Parse: (malformed, %d bytes)", connID, n)
	//			}
	//		} else if data[0] == 'B' && n > 5 {
	//			params, err := parseBindParameters(data, n)
	//			if err != nil {
	//				log.Printf("FROM-CLIENT; [Conn %d] Client Bind: failed to parse parameters: %v", connID, err)
	//			} else {
	//				log.Printf("FROM-CLIENT; [Conn %d] Client Bind Parameters: %v", connID, params)
	//			}
	//		} else if data[0] == 'p' {
	//			log.Printf("FROM-CLIENT; [Conn %d] Client Authentication: (password or SASL data)", connID)
	//		} else if data[0] == 'D' && n > 5 {
	//			log.Printf("FROM-CLIENT; [Conn %d] Client Describe: %s", connID, parseDescribeMessage(data))
	//		} else if data[0] == 'E' && n == 5 {
	//			log.Printf("FROM-CLIENT; [Conn %d] Client Execute", connID)
	//		} else if data[0] == 'S' && n == 5 {
	//			log.Printf("FROM-CLIENT; [Conn %d] Client Sync", connID)
	//		} else if data[0] == 'X' && n == 5 {
	//			log.Printf("FROM-CLIENT; [Conn %d] Client Terminate", connID)
	//		} else {
	//			log.Printf("FROM-CLIENT; [Conn %d] Client -> PostgreSQL: %x", connID, data)
	//		}
	//
	//		// Forward data to PostgreSQL
	//		_, err = pgConn.Write(data)
	//		if err != nil {
	//			log.Printf("[Conn %d] Error forwarding to PostgreSQL: %v", connID, err)
	//			return
	//		}
	//	}
	//}()

	go func() {
		defer wg.Done()
		reader := bufio.NewReader(clientConn)
		isStartup := true
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
			} else if data[0] == 'Q' && n > 5 {
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
			_, err = pgConn.Write(data)
			if err != nil {
				log.Printf("[Conn %d] Error forwarding to PostgreSQL: %v", connID, err)
				return
			}
		}
	}()

	// PostgreSQL to Client with logging
	go func() {
		defer wg.Done()

		reader := bufio.NewReader(pgConn)

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
	}()

	wg.Wait()
	log.Printf("[Conn %d] Connection closed", connID)
}

// parseStartupMessage extracts parameters from startup packet
func parseStartupMessage(data []byte) map[string]string {
	params := make(map[string]string)
	parts := bytes.Split(data, []byte{0})
	for i := 0; i < len(parts)-1; i += 2 {
		key := string(parts[i])
		value := string(parts[i+1])
		if key != "" {
			params[key] = value
		}
	}
	return params
}

// parseErrorOrNotice extracts fields from ErrorResponse or NoticeResponse
func parseErrorOrNotice(data []byte) map[string]string {
	fields := make(map[string]string)
	parts := bytes.Split(data, []byte{0})
	for _, part := range parts {
		if len(part) > 1 {
			fields[string(part[0])] = string(part[1:])
		}
	}
	return fields
}

func checkAndLogQueryType(connID int, query string) {
	q := strings.TrimSpace(strings.ToUpper(query))

	switch {
	case strings.HasPrefix(q, "SELECT"):
		log.Printf("FROM-CLIENT; [Conn %d] --> SELECT detected", connID)
	case strings.HasPrefix(q, "INSERT"):
		log.Printf("FROM-CLIENT; [Conn %d] --> INSERT detected", connID)
	case strings.HasPrefix(q, "UPDATE"):
		log.Printf("FROM-CLIENT; [Conn %d] --> UPDATE detected", connID)
	case strings.HasPrefix(q, "DELETE"):
		log.Printf("FROM-CLIENT; [Conn %d] --> DELETE detected", connID)
	case strings.HasPrefix(q, "CREATE"):
		log.Printf("FROM-CLIENT; [Conn %d] --> CREATE detected", connID)
	default:
		// Non-filtered queries
	}
}

// parseRowDescription extracts column names from RowDescription
func parseRowDescription(data []byte) []string {
	var columns []string
	if len(data) < 2 {
		return columns
	}
	count := int(binary.BigEndian.Uint16(data[0:2]))
	pos := 2
	for i := 0; i < count && pos < len(data); i++ {
		end := bytes.IndexByte(data[pos:], 0)
		if end == -1 {
			break
		}
		columns = append(columns, string(data[pos:pos+end]))
		pos += end + 19 // Skip name + metadata
	}
	return columns
}

// parseDataRow extracts values from DataRow
func parseDataRow(data []byte) []string {
	var values []string
	if len(data) < 2 {
		return values
	}
	count := int(binary.BigEndian.Uint16(data[0:2]))
	pos := 2
	for i := 0; i < count && pos < len(data); i++ {
		if pos+4 > len(data) {
			values = append(values, "(truncated)")
			break
		}
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if length == -1 {
			values = append(values, "NULL")
		} else if pos+length <= len(data) {
			values = append(values, string(data[pos:pos+length]))
			pos += length
		} else {
			values = append(values, "(truncated)")
			break
		}
	}
	return values
}

// parseParameterStatus extracts key-value pair from ParameterStatus
func parseParameterStatus(data []byte) [2]string {
	parts := bytes.SplitN(data, []byte{0}, 3)
	if len(parts) < 2 {
		return [2]string{"unknown", "unknown"}
	}
	return [2]string{string(parts[0]), string(parts[1])}
}

// parseBindParameters extracts parameter values from Bind message
func parseBindParameters(data []byte, n int) ([]string, error) {
	var values []string
	pos := bytes.IndexByte(data, 0) + 1 // Skip portal name
	if pos >= n {
		return nil, fmt.Errorf("invalid bind message: no portal name terminator")
	}
	pos += bytes.IndexByte(data[pos:], 0) + 1 // Skip statement name
	if pos >= n {
		return nil, fmt.Errorf("invalid bind message: no statement name terminator")
	}
	if pos+2 > n {
		return nil, fmt.Errorf("invalid bind message: too short for parameter count")
	}
	numParams := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	for i := 0; i < numParams && pos < n; i++ {
		if pos+4 > n {
			return values, fmt.Errorf("invalid bind message: truncated parameter length at index %d", i)
		}
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if length == -1 {
			values = append(values, "NULL")
		} else if length < 0 || pos+length > n {
			return values, fmt.Errorf("invalid bind message: parameter %d length %d invalid or exceeds buffer (%d)", i, length, n)
		} else {
			values = append(values, string(data[pos:pos+length]))
			pos += length
		}
	}
	return values, nil
}

// parseDescribeMessage extracts portal or statement name from Describe message
func parseDescribeMessage(data []byte) string {
	if len(data) < 6 {
		return "unknown"
	}
	kind := data[5]
	name := string(bytes.Trim(data[6:], "\x00"))
	if kind == 'S' {
		return fmt.Sprintf("Statement %s", name)
	} else if kind == 'P' {
		return fmt.Sprintf("Portal %s", name)
	}
	return "unknown"
}

type Record struct {
	Length int
	Type   int
	ID     int
	Code   string
	String string
}

func parseHexRecords(hexStr string) ([]Record, error) {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}

	var records []Record
	offset := 0

	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+2 > len(data) {
			break
		}
		typeField := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if offset+4 > len(data) {
			break
		}
		idField := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+3 > len(data) {
			break
		}
		code := string(data[offset : offset+3])
		offset += 3

		if offset+4 > len(data) {
			break
		}
		strLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+strLen > len(data) {
			break
		}
		stringVal := string(data[offset : offset+strLen])
		offset += strLen

		records = append(records, Record{
			Length: length,
			Type:   typeField,
			ID:     idField,
			Code:   code,
			String: stringVal,
		})
	}

	return records, nil
}

func parsePostgresDataRow(data []byte) (string, error) {
	buf := bytes.NewReader(data)

	// Message type
	var msgType byte
	if err := binary.Read(buf, binary.BigEndian, &msgType); err != nil {
		return "", err
	}
	if msgType != 'D' {
		return "", fmt.Errorf("not a DataRow message")
	}

	// Message length
	var length int32
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return "", err
	}

	// Column count
	var colCount int16
	if err := binary.Read(buf, binary.BigEndian, &colCount); err != nil {
		return "", err
	}

	values := make([]string, 0, colCount)
	for i := 0; i < int(colCount); i++ {
		var valLen int32
		if err := binary.Read(buf, binary.BigEndian, &valLen); err != nil {
			return "", err
		}

		if valLen == -1 { // NULL value
			values = append(values, "NULL")
			continue
		}

		val := make([]byte, valLen)
		if _, err := buf.Read(val); err != nil {
			return "", err
		}
		values = append(values, string(val))
	}

	return strings.Join(values, ", "), nil
}

func main() {
	listenAddr := flag.String("listen", "localhost:5433", "Address for proxy to listen on")
	pgAddr := flag.String("pg", "localhost:5432", "PostgreSQL server address")
	flag.Parse()

	config := &Config{
		listenAddr: *listenAddr,
		pgAddr:     *pgAddr,
	}

	proxy := NewProxy(config)
	if err := proxy.Start(); err != nil {
		log.Fatalf("Proxy failed: %v", err)
	}
}
