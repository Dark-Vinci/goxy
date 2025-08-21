package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

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

// parseBindParameters parses a PostgresSQL Bind message into a slice of parameter strings.
func parseBindParameters(data []byte) ([]string, string, error) {
	if len(data) < 6 || data[0] != 'B' {
		return nil, "", fmt.Errorf("not a Bind message")
	}

	// skip type + length
	pos := 5

	// read portal name (null-terminated)
	end := bytes.IndexByte(data[pos:], 0)
	if end < 0 {
		return nil, "", fmt.Errorf("invalid portal name")
	}
	portal := string(data[pos : pos+end])
	pos += end + 1

	fmt.Println("PORTAL: ", portal)

	// read statement name (null-terminated)
	end = bytes.IndexByte(data[pos:], 0)
	if end < 0 {
		return nil, "", fmt.Errorf("invalid statement name")
	}
	stmtName := string(data[pos : pos+end])
	pos += end + 1

	// read a number of format codes
	if pos+2 > len(data) {
		return nil, stmtName, fmt.Errorf("truncated bind message (format count)")
	}
	nFormats := int(binary.BigEndian.Uint16(data[pos:]))
	pos += 2 + (nFormats * 2) // skip format codes

	// number of parameters
	if pos+2 > len(data) {
		return nil, stmtName, fmt.Errorf("truncated bind message (param count)")
	}
	nParams := int(binary.BigEndian.Uint16(data[pos:]))
	pos += 2

	params := make([]string, nParams)

	for i := 0; i < nParams; i++ {
		if pos+4 > len(data) {
			return nil, stmtName, fmt.Errorf("truncated bind message (param length)")
		}
		length := int(binary.BigEndian.Uint32(data[pos:]))
		pos += 4

		if length == -1 {
			params[i] = "NULL"
		} else {
			if pos+length > len(data) {
				return nil, stmtName, fmt.Errorf("truncated bind message (param value)")
			}
			val := string(data[pos : pos+length])

			fmt.Println("RAWWWWW", val)
			// naive quoting for logging
			params[i] = "'" + strings.ReplaceAll(val, "'", "''") + "'"

			fmt.Println("UNRAWWWWW", params[i])
			pos += length
		}
	}

	return params, stmtName, nil
}

// parseDescribeMessage extracts portal or statement name from a Describe message
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
