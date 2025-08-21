package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	// Connection string to connect to the proxy
	connStr := "host=localhost port=5435 user=appuser password=apppass dbname=appdb token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NTU4NzE4MDIsInJvbGUiOiJhZG1pbiIsInVzZXJuYW1lIjoibWVsb24ifQ.bqOod35kR4wssBq0UylZKvlRkglOazQL1GkIasyUtAY sslmode=disable"

	// Open a connection to the proxy
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open connection: %v", err)
	}
	defer db.Close()

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Successfully connected to the database through the proxy")

	// Execute a sample query
	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	fmt.Printf("Query result: %d\n", result)

	// Example: Create a table and insert data
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
id SERIAL PRIMARY KEY,
name TEXT NOT NULL
)`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	fmt.Println("Created users table")

	//for i := 0; i < 100; i++ {
	name := fmt.Sprintf("Alice-%d", 1)
	_, err = db.Exec("INSERT INTO users (name) VALUES ($1)", name)
	if err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}
	fmt.Println("Inserted user:", name)
	//}

	// Query the table
	rows, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}
	defer rows.Close()
	//
	fmt.Println("Users in the database:")
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		fmt.Printf("ID: %d, Name: %s\n", id, name)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("Row iteration error: %v", err)
	}
}
