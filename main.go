package main

import (
	"flag"
	"log"
)

func main() {
	listenAddr := flag.String("listen", "localhost:5433", "Address for proxy to listen on")
	pgAddr := flag.String("pg", "localhost:5432", "PostgreSQL server address")
	flag.Parse()

	config := &Config{
		listenAddr: *listenAddr,
		master:     *pgAddr,
		slaves:     []string{},
	}

	proxy := NewProxy(config)
	if err := proxy.Start(); err != nil {
		log.Fatalf("Proxy failed: %v", err)
	}
}

//type Record struct {
//	Length int
//	Type   int
//	ID     int
//	Code   string
//	String string
//}
//
//func parseHexRecords(hexStr string) ([]Record, error) {
//	data, err := hex.DecodeString(hexStr)
//	if err != nil {
//		return nil, err
//	}
//
//	var records []Record
//	offset := 0
//
//	for offset < len(data) {
//		if offset+4 > len(data) {
//			break
//		}
//		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
//		offset += 4
//
//		if offset+2 > len(data) {
//			break
//		}
//		typeField := int(binary.BigEndian.Uint16(data[offset : offset+2]))
//		offset += 2
//
//		if offset+4 > len(data) {
//			break
//		}
//		idField := int(binary.BigEndian.Uint32(data[offset : offset+4]))
//		offset += 4
//
//		if offset+3 > len(data) {
//			break
//		}
//		code := string(data[offset : offset+3])
//		offset += 3
//
//		if offset+4 > len(data) {
//			break
//		}
//		strLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
//		offset += 4
//
//		if offset+strLen > len(data) {
//			break
//		}
//		stringVal := string(data[offset : offset+strLen])
//		offset += strLen
//
//		records = append(records, Record{
//			Length: length,
//			Type:   typeField,
//			ID:     idField,
//			Code:   code,
//			String: stringVal,
//		})
//	}
//
//	return records, nil
//}
