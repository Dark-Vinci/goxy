package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"strings"
)

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
