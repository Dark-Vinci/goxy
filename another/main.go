package another

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// UpstreamRole defines the role of an upstream
type UpstreamRole string

const (
	RolePrimary UpstreamRole = "primary"
	RoleReplica UpstreamRole = "replica"
)

// UserRole defines user roles for RBAC
type UserRole string

const (
	UserRoleAdmin     UserRole = "admin"
	UserRoleReadWrite UserRole = "read_write"
	UserRoleReadOnly  UserRole = "read_only"
)

// Config holds proxy configuration
type Config struct {
	Master string
	Slaves []string
	Listen string // e.g., "0.0.0.0:5433"
}

// Session holds the upstream connections
type Session struct {
	UpPrimary *Upstream
	Replicas  []*Upstream
}

// Upstream represents an upstream server
type Upstream struct {
	Addr    string
	Role    UpstreamRole
	Healthy bool
	Lag     int
	Conn    *pgx.Conn
	lock    sync.Mutex
}

// String implements the fmt.Stringer interface for Upstream
func (u *Upstream) String() string {
	u.lock.Lock()
	defer u.lock.Unlock()
	return fmt.Sprintf("Upstream{Addr: %s, Role: %s, Healthy: %v, Lag: %dms}", u.Addr, u.Role, u.Healthy, u.Lag)
}

// Close closes the upstream connection safely
func (u *Upstream) Close() error {
	u.lock.Lock()
	defer u.lock.Unlock()
	if u.Conn != nil {
		err := u.Conn.Close(context.Background())
		u.Conn = nil
		return err
	}
	return nil
}

// Proxy manages the proxy instance
type Proxy struct {
	config   *Config
	logger   *zerolog.Logger
	sqliteDB *sql.DB
	session  *Session
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewProxy creates a new Proxy instance
func NewProxy(config *Config, db *sql.DB, logger zerolog.Logger) *Proxy {
	session := &Session{}
	openedConns := make([]*pgx.Conn, 0)

	// Connect to master (adjust credentials)
	masterConn, err := pgx.Connect(context.Background(), fmt.Sprintf("postgres://user:password@%s/postgres", config.Master))
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to connect to master %s: %v", config.Master, err)
		return nil
	}
	openedConns = append(openedConns, masterConn)

	session.UpPrimary = &Upstream{
		Addr:    config.Master,
		Role:    RolePrimary,
		Healthy: true,
		Lag:     0,
		Conn:    masterConn,
		lock:    sync.Mutex{},
	}

	// Connect to replicas
	for _, v := range config.Slaves {
		replicaConn, err := pgx.Connect(context.Background(), fmt.Sprintf("postgres://user:password@%s/postgres", v))
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to connect to replica %s: %v", v, err)
			for _, conn := range openedConns {
				conn.Close(context.Background())
			}
			return nil
		}
		openedConns = append(openedConns, replicaConn)

		session.Replicas = append(session.Replicas, &Upstream{
			Addr:    v,
			Role:    RoleReplica,
			Healthy: false,
			Lag:     0,
			Conn:    replicaConn,
			lock:    sync.Mutex{},
		})
	}

	ctx, cancel := context.WithCancel(context.Background())

	proxy := &Proxy{
		config:   config,
		logger:   &logger,
		sqliteDB: db,
		session:  session,
		ctx:      ctx,
		cancel:   cancel,
	}

	proxy.startPinging()
	return proxy
}

// Close shuts down the Proxy and closes all connections
func (p *Proxy) Close() error {
	p.cancel()
	var errs []error
	if err := p.session.UpPrimary.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close master %s: %v", p.session.UpPrimary.Addr, err))
	}
	for _, replica := range p.session.Replicas {
		if err := replica.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close replica %s: %v", replica.Addr, err))
		}
	}
	if err := p.sqliteDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close database: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}
	return nil
}

// PingUpstream sends a ping request to the upstream every minute and updates the database
func (p *Proxy) PingUpstream(upstream *Upstream) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info().Msgf("Stopping pinging for %s", upstream)
			return
		case <-ticker.C:
			upstream.lock.Lock()
			healthy := false
			lag := 0

			if upstream.Conn != nil {
				start := time.Now()
				err := upstream.Conn.Ping(p.ctx)
				if err == nil {
					healthy = true
					lag = int(time.Since(start).Milliseconds())
				} else {
					p.logger.Warn().Err(err).Msgf("Ping failed for %s: %v", upstream, err)
					// Attempt reconnect
					upstream.Conn.Close(p.ctx)
					upstream.Conn, err = pgx.Connect(p.ctx, fmt.Sprintf("postgres://user:password@%s/postgres", upstream.Addr))
					if err != nil {
						p.logger.Warn().Err(err).Msgf("Failed to reconnect to %s", upstream)
					}
				}
			}

			// Sanitize addr for SQLite
			addr := strings.ReplaceAll(upstream.Addr, ":", "_")
			_, err := p.sqliteDB.Exec("UPDATE upstreams SET healthy = ?, lag = ? WHERE addr = ?",
				healthy, lag, addr)
			if err != nil {
				p.logger.Warn().Err(err).Msgf("Failed to update %s in database: %v", upstream, err)
			} else {
				p.logger.Info().Msgf("Pinged %s: healthy=%v, lag=%dms", upstream, healthy, lag)
			}

			upstream.Healthy = healthy
			upstream.Lag = lag
			upstream.lock.Unlock()
		}
	}
}

// startPinging starts pinging all upstreams
func (p *Proxy) startPinging() {
	go p.PingUpstream(p.session.UpPrimary)
	for _, replica := range p.session.Replicas {
		go p.PingUpstream(replica)
	}
}

// Serve starts the proxy server, accepting client connections and enforcing RBAC
func (p *Proxy) Serve() error {
	listener, err := net.Listen("tcp", p.config.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.config.Listen, err)
	}
	defer listener.Close()

	p.logger.Info().Msgf("Proxy listening on %s", p.config.Listen)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			p.logger.Warn().Err(err).Msg("Failed to accept client connection")
			continue
		}
		go p.handleClient(clientConn)
	}
}

// handleClient handles a client connection with authentication and RBAC
func (p *Proxy) handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	backend := pgproto3.NewBackend(clientConn, clientConn)
	frontend := pgproto3.NewFrontend(clientConn, clientConn)

	// Receive startup message
	startupMsg, err := backend.ReceiveStartupMessage()
	if err != nil {
		p.logger.Warn().Err(err).Msg("Failed to receive startup message")
		return
	}

	// Parse username from startup parameters
	params, ok := startupMsg.(*pgproto3.StartupMessage)
	if !ok {
		p.logger.Warn().Msg("Invalid startup message")
		return
	}
	username, ok := params.Parameters["user"]
	if !ok {
		p.logger.Warn().Msg("No username in startup message")
		return
	}

	// Send authentication request (MD5 password challenge)
	salt := [4]byte{0x01, 0x02, 0x03, 0x04} // Random salt in production
	authMD5 := &pgproto3.AuthenticationMD5Password{Salt: salt}
	if err := frontend.Send(authMD5); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to send auth challenge")
		return
	}

	// Receive password message
	msg, err := backend.Receive()
	if err != nil {
		p.logger.Warn().Err(err).Msg("Failed to receive password")
		return
	}
	passwordMsg, ok := msg.(*pgproto3.PasswordMessage)
	if !ok {
		p.logger.Warn().Msg("Invalid password message")
		return
	}

	// Verify password against SQLite (retrieve hashed password)
	var storedHash string
	var role UserRole
	err = p.sqliteDB.QueryRow("SELECT password, role FROM users WHERE username = ?", username).Scan(&storedHash, &role)
	if err != nil {
		if err == sql.ErrNoRows {
			p.logger.Warn().Msgf("User not found: %s", username)
		} else {
			p.logger.Warn().Err(err).Msgf("Failed to query user %s", username)
		}
		frontend.Send(&pgproto3.ErrorResponse{Message: "authentication failed"})
		return
	}

	// Verify MD5 hashed password (PostgreSQL MD5 format: md5(password + username) with salt)
	md5Pass := md5.Sum([]byte(passwordMsg.Password + username))
	md5WithSalt := md5.Sum(append(md5Pass[:], salt...))
	if fmt.Sprintf("%x", md5WithSalt) != storedHash {
		p.logger.Warn().Msgf("Invalid password for user %s", username)
		frontend.Send(&pgproto3.ErrorResponse{Message: "authentication failed"})
		return
	}

	// Authentication success
	frontend.Send(&pgproto3.AuthenticationOk{})
	frontend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})

	p.logger.Info().Msgf("User %s authenticated with role %s", username, role)

	// Now proxy messages with RBAC
	for {
		msg, err := backend.Receive()
		if err != nil {
			p.logger.Warn().Err(err).Msg("Failed to receive client message")
			return
		}

		switch m := msg.(type) {
		case *pgproto3.Query:
			// Simple query; classify as read or write
			query := strings.ToUpper(strings.TrimSpace(m.String))
			isWrite := strings.HasPrefix(query, "INSERT") || strings.HasPrefix(query, "UPDATE") || strings.HasPrefix(query, "DELETE") ||
				strings.HasPrefix(query, "CREATE") || strings.HasPrefix(query, "DROP") || strings.HasPrefix(query, "ALTER")

			// Enforce RBAC
			if role == UserRoleReadOnly && isWrite {
				frontend.Send(&pgproto3.ErrorResponse{Message: "write operation not allowed for read_only role"})
				continue
			}

			// Route based on role and query type
			var target *Upstream
			if isWrite || role == UserRoleAdmin || role == UserRoleReadWrite {
				target = p.selectHealthyMaster()
			} else {
				target = p.selectHealthyReplica()
			}

			if target == nil {
				frontend.Send(&pgproto3.ErrorResponse{Message: "no healthy upstream available"})
				continue
			}

			// Forward to target upstream (using pgx to execute)
			target.lock.Lock()
			rows, err := target.Conn.Query(p.ctx, m.String)
			target.lock.Unlock()
			if err != nil {
				frontend.Send(&pgproto3.ErrorResponse{Message: err.Error()})
				continue
			}
			defer rows.Close()

			// Send results back to client
			for rows.Next() {
				values, err := rows.Values()
				if err != nil {
					frontend.Send(&pgproto3.ErrorResponse{Message: err.Error()})
					return
				}
				// Send RowDescription and DataRow (simplified; use pgproto3 to construct)
				// In production, fully serialize the response using pgproto3
				p.logger.Info().Msgf("Row: %v", values)
			}
			frontend.Send(&pgproto3.CommandComplete{CommandTag: "SELECT"})
			frontend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})

		case *pgproto3.Terminate:
			return
		default:
			// Forward other messages if needed
		}
	}
}

// selectHealthyMaster returns the master if healthy
func (p *Proxy) selectHealthyMaster() *Upstream {
	p.session.UpPrimary.lock.Lock()
	defer p.session.UpPrimary.lock.Unlock()
	if p.session.UpPrimary.Healthy {
		return p.session.UpPrimary
	}
	return nil
}

// selectHealthyReplica returns a healthy replica (simple round-robin or random in production)
func (p *Proxy) selectHealthyReplica() *Upstream {
	for _, r := range p.session.Replicas {
		r.lock.Lock()
		if r.Healthy {
			r.lock.Unlock()
			return r
		}
		r.lock.Unlock()
	}
	return nil
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	if err := godotenv.Load(); err != nil {
		logger.Fatal().Err(err).Msg("Error loading .env file")
	}

	masterAddr := os.Getenv("MASTER")
	if masterAddr == "" {
		logger.Fatal().Msg("MASTER environment variable not set")
	}

	slavesStr := os.Getenv("SLAVES")
	var slaves []string
	if slavesStr != "" {
		slaves = strings.Split(slavesStr, ",")
		for i, s := range slaves {
			slaves[i] = strings.TrimSpace(s)
		}
	}

	config := &Config{
		Master: masterAddr,
		Slaves: slaves,
		Listen: "0.0.0.0:5433", // Proxy listen address
	}

	db, err := sql.Open("sqlite3", "./upstream.db")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}

	// Create upstreams table
	createUpstreamsSQL := `
	CREATE TABLE IF NOT EXISTS upstreams (
		addr TEXT PRIMARY KEY,
		role TEXT,
		healthy BOOLEAN,
		lag INTEGER
	);`
	_, err = db.Exec(createUpstreamsSQL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create upstreams table")
	}

	// Create users table for RBAC
	createUsersSQL := `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password TEXT,
		role TEXT
	);`
	_, err = db.Exec(createUsersSQL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create users table")
	}

	// Insert sample users (hash passwords; in production, use a secure method to add users)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	_, err = db.Exec("INSERT OR REPLACE INTO users (username, password, role) VALUES (?, ?, ?)",
		"admin", hashedPassword, UserRoleAdmin)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to insert admin user")
	}
	hashedPassword, _ = bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	_, err = db.Exec("INSERT OR REPLACE INTO users (username, password, role) VALUES (?, ?, ?)",
		"reader", hashedPassword, UserRoleReadOnly)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to insert reader user")
	}

	// Insert upstreams with sanitized addr
	_, err = db.Exec("INSERT OR REPLACE INTO upstreams (addr, role, healthy, lag) VALUES (?, ?, ?, ?)",
		strings.ReplaceAll(config.Master, ":", "_"), RolePrimary, true, 0)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to insert master %s", config.Master)
	}
	for _, slave := range config.Slaves {
		_, err = db.Exec("INSERT OR REPLACE INTO upstreams (addr, role, healthy, lag) VALUES (?, ?, ?, ?)",
			strings.ReplaceAll(slave, ":", "_"), RoleReplica, false, 0)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to insert replica %s", slave)
		}
	}

	proxy := NewProxy(config, db, logger)
	if proxy == nil {
		logger.Fatal().Msg("Failed to create proxy")
	}
	defer func() {
		if err := proxy.Close(); err != nil {
			logger.Error().Err(err).Msg("Failed to close proxy")
		}
	}()

	// Start the proxy server
	if err := proxy.Serve(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start proxy server")
	}
}
