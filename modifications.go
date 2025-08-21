package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)

func readStartupMessage(conn net.Conn) ([]byte, error) {
	// First 4 bytes: length
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	// Read rest
	msg := make([]byte, length-4)
	if _, err := io.ReadFull(conn, msg); err != nil {
		return nil, err
	}

	return append(lenBuf, msg...), nil
}

func parseTheStartupMessage(msg []byte) (map[string]string, uint32) {
	protocol := binary.BigEndian.Uint32(msg[4:8])
	params := map[string]string{}

	i := 8
	for i < len(msg)-1 {
		// find key
		endKey := bytes.IndexByte(msg[i:], 0)
		if endKey == -1 || i+endKey+1 >= len(msg) {
			break
		}
		key := string(msg[i : i+endKey])
		i += endKey + 1

		// find value
		endVal := bytes.IndexByte(msg[i:], 0)
		if endVal == -1 {
			break
		}
		val := string(msg[i : i+endVal])
		i += endVal + 1

		params[key] = val
	}
	return params, protocol
}

func buildStartupMessage(params map[string]string, protocol uint32) []byte {
	buf := new(bytes.Buffer)

	// placeholder for length
	binary.Write(buf, binary.BigEndian, uint32(0))
	binary.Write(buf, binary.BigEndian, protocol)

	for k, v := range params {
		buf.WriteString(k)
		buf.WriteByte(0)
		buf.WriteString(v)
		buf.WriteByte(0)
	}
	buf.WriteByte(0) // terminator

	// fix length
	msg := buf.Bytes()
	binary.BigEndian.PutUint32(msg[0:4], uint32(len(msg)))
	return msg
}

func writeError(conn net.Conn, severity, code, msg string) error {
	buf := new(bytes.Buffer)

	// Type
	buf.WriteByte('E')

	// Placeholder for length
	binary.Write(buf, binary.BigEndian, int32(0))

	// Fields
	buf.WriteByte('S')
	buf.WriteString(severity)
	buf.WriteByte(0)

	buf.WriteByte('C')
	buf.WriteString(code)
	buf.WriteByte(0)

	buf.WriteByte('M')
	buf.WriteString(msg)
	buf.WriteByte(0)

	buf.WriteByte(0) // terminator

	// Fix length
	data := buf.Bytes()
	length := int32(len(data) - 1) // length excludes type byte
	binary.BigEndian.PutUint32(data[1:5], uint32(length))

	_, err := conn.Write(data)
	return err
}
