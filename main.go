package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	appLogger := logger.With().Str("Thesis", "api").Logger()

	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	db, err := sql.Open("sqlite3", "./upstream.db")
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to open database: %v", err)
		return
	}

	defer func(db *sql.DB) {
		if err := db.Close(); err != nil {
			logger.Fatal().Err(err).Msgf("Failed to close database: %v", err)
		}
	}(db)

	config := NewConfig()

	proxy := NewProxy(config, db, appLogger)
	if err := proxy.Start(context.Background()); err != nil {
		logger.Fatal().Err(err).Msgf("Failed to start proxy: %v", err)
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
