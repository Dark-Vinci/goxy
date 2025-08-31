package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

func (p *Proxy) frontend(serverConn net.Conn, request *Request, connID int, role UserRole, wg *sync.WaitGroup) {
	defer wg.Done()

	var (
		preparedStatement string
		bindParameters    []string
		reader            = bufio.NewReader(request.conn)
		sqls              = make([]SQL, 0)
	)

	defer func() {
		request.Sql = sqls
		now := time.Now()
		request.CompletedAt = &now
	}()

	for {
		fmt.Println("HERE")
		// Read data from a client
		data := make([]byte, 16384)
		var sql SQL
		n, err := reader.Read(data)
		fmt.Println("HERE1")
		if err != nil {
			if err != io.EOF {
				p.logger.Error().Err(err).Msgf("FROM-CLIENT; [Conn %d] Error reading from client: %v", connID, err)
			}
			return
		}

		if n == 0 {
			continue
		}

		fmt.Println("HERE2")

		data = data[:n]

		if data[0] == 'Q' && n > 5 { // here
			query := string(bytes.Trim(data[5:], "\x00"))
			p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Query: %s", connID, query)
			fmt.Println("HERE3")
			sql.Sql = query
		} else if data[0] == 'P' && n > 5 {
			idx := bytes.IndexByte(data[5:], 0) + 5
			if idx < n-1 {
				query := string(bytes.Trim(data[idx+1:bytes.IndexByte(data[idx+1:], 0)+idx+1], "\x00"))
				p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Parse: %s", connID, query)

				preparedStatement = query
			} else {
				p.logger.Warn().Msgf("FROM-CLIENT; [Conn %d] Client Parse: (malformed, %d bytes)", connID, n)
			}

			fmt.Println("HERE3")
		} else if data[0] == 'B' && n > 5 {
			params, _, err := parseBindParameters(data)
			if err != nil {
				p.logger.Warn().Err(err).Msgf("FROM-CLIENT; [Conn %d] Client Bind: failed to parse parameters: %v", connID, err)
			} else {
				bindParameters = params
				p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Bind Parameters: %v", connID, params)
			}

			fmt.Println("HERE4")
		} else if data[0] == 'p' {
			fmt.Println("HERE5")
			p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Password", connID)
		} else if data[0] == 'D' && n > 5 {
			fmt.Println("HERE6")
			p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Describe %v", connID, parseDescribeMessage(data))
		} else if data[0] == 'E' && n == 5 {
			fmt.Println("HERE7")
			p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Close", connID)
		} else if data[0] == 'S' && n == 5 {
			fmt.Println("HERE8")
			p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Sync", connID)
		} else if data[0] == 'X' && n == 5 {
			fmt.Println("HERE9")
			p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client Terminate", connID)
		} else {
			fmt.Println("HERE10")
			p.logger.Info().Msgf("FROM-CLIENT; [Conn %d] Client -> PostgreSQL: %x", connID, data)
		}

		fmt.Println("HERE11")

		//TODO; REQUIRES MORE INDEPTH CHECKS
		if len(bindParameters) > 0 {
			// Naive substitution: replace $1, $2, ... with params
			for i, param := range bindParameters {
				placeholder := fmt.Sprintf("$%d", i+1)
				var value string
				//switch v := param.(type) {
				//case string:
				// value = fmt.Sprintf("'%s'", strings.ReplaceAll(, "'", "''")) // escape quotes
				//default:
				value = fmt.Sprintf("%v", param)
				//}
				preparedStatement = strings.Replace(preparedStatement, placeholder, value, 1)
			}

			sql.Sql = preparedStatement

			bindParameters = nil
			preparedStatement = ""
		}

		fmt.Println("HERE12")

		// IF THE LENGTH OF QUERY STRING IS MORE THAN 0 -> INSERT INTO DB AND CONTINUE
		if len(sql.Sql) > 1 {
			queryType := p.classifyQuery(sql.Sql)

			sql.IsRead = queryType == QueryRead

			if queryType == QueryWrite && role == UserRoleReadOnly {
				p.logger.Info().Msgf("user doesn't have write access for SQL: %s", sql.Sql)

				//sql.

				// write error and quit
				// quit
				//STOP
			}
		}

		fmt.Println("I AM YOUNG CAT")

		// Forward data to PostgresSQL
		_, err = serverConn.Write(data)
		if err != nil {
			p.logger.Error().Err(err).Msgf("FROM-CLIENT; [Conn %d] Error forwarding to PostgreSQL: %v", connID, err)
			return
		}

		fmt.Println("I AM YOUNG CAT1111")

		now := time.Now()
		sql.CompletedAt = &now

		if len(sql.Sql) > 1 {
			sqls = append(sqls, sql)
		}
	}
}

func (p *Proxy) backend(serverConn, clientConn net.Conn, connID int, wg *sync.WaitGroup) {
	defer wg.Done()

	var (
		reader = bufio.NewReader(serverConn)
	)

	for {
		fmt.Println("THERE")
		// Read message type (1 byte)
		msgType, err := reader.ReadByte()
		if err != nil {
			if err != io.EOF {
				p.logger.Error().Err(err).Msgf("FROM-POSTGRES; [Conn %d] Error reading message type from PostgreSQL: %v", connID, err)
			}
			return
		}

		// Read length (int32, includes length bytes but not the type byte)
		lengthBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, lengthBytes); err != nil {
			p.logger.Error().Err(err).Msgf("FROM-POSTGRES; [Conn %d] Error reading message length from PostgreSQL: %v", connID, err)
			return
		}

		msgLength := int(binary.BigEndian.Uint32(lengthBytes))
		if msgLength < 4 {
			p.logger.Error().Msgf("FROM-POSTGRES; [Conn %d] Invalid message length: %d", connID, msgLength)
			return
		}

		// Read the rest of the message body
		body := make([]byte, msgLength-4)
		if _, err := io.ReadFull(reader, body); err != nil {
			p.logger.Error().Err(err).Msgf("FROM-POSTGRES; [Conn %d] Error reading message body from PostgreSQL: %v", connID, err)
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
					p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Authentication: Success", connID)
				case 10:
					sasl := string(bytes.Trim(body[4:], "\x00"))
					p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Authentication: SASL requested (%s)", connID, sasl)
				default:
					p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Authentication: Type %d", connID, authType)
				}
			}
		case 'C':
			tag := string(bytes.Trim(body, "\x00"))
			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Command Complete: %s", connID, tag)
		case 'E':
			fields := parseErrorOrNotice(body)
			p.logger.Warn().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Error: %s", connID, fields["M"])
		case 'N':
			fields := parseErrorOrNotice(body)
			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Notice: %s", connID, fields["M"])
		case 'T':
			columns := parseRowDescription(body)
			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Row Description: %v", connID, columns)
		case 'D':
			values := parseDataRow(body)

			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Data Row: %v", connID, values)
		case 'Z':
			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Ready for Query", connID)
		case 'S':
			keyValue := parseParameterStatus(body)
			if len(keyValue) >= 2 {
				p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Parameter Status: %s=%s", connID, keyValue[0], keyValue[1])
			}
		case 'K':
			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Backend Key Data", connID)
		case '1':
			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Parse Complete", connID)
		case '2':
			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Bind Complete", connID)
		default:
			records, err := parsePostgresDataRow(body)
			if err != nil {
				p.logger.Warn().Err(err).Msgf("FROM-POSTGRES; [Conn %d] Error parsing hex records: %v", connID, err)
			} else {
				p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL Data Row: %v", connID, records)
			}

			p.logger.Info().Msgf("FROM-POSTGRES; [Conn %d] PostgreSQL -> Client: %x", connID, fullMsg)
		}

		// Forward the message to the client
		if _, err = clientConn.Write(fullMsg); err != nil {
			p.logger.Error().Err(err).Msgf("FROM-POSTGRES; [Conn %d] Error forwarding to client: %v", connID, err)
			return
		}
	}
}
