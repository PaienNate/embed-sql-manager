package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"embedSqlClient/sqlui"
)

// The example's datacenterDB (PostgreSQL) connection reads its settings from
// the environment, so no real host, database or password is committed. Set
// POSTGRES_HOST (plus the optional POSTGRES_PORT / POSTGRES_USER /
// POSTGRES_PASSWORD / POSTGRES_DBNAME / POSTGRES_CONFIG) to enable it. When
// POSTGRES_HOST is empty the connection is skipped.
func postgresDSN() string {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		return "" // not configured -> datacenterDB disabled
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("POSTGRES_PASSWORD")
	database := os.Getenv("POSTGRES_DBNAME")
	if database == "" {
		database = "postgres"
	}
	config := os.Getenv("POSTGRES_CONFIG")
	if config == "" {
		config = "sslmode=disable connect_timeout=10"
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s %s",
		host, port, user, password, database, config,
	)
}

func main() {
	dbPath := os.Getenv("SQLUI_DB")
	if dbPath == "" {
		dbPath = "demo.db"
	}
	addr := os.Getenv("SQLUI_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	p, err := openSQLite(dbPath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer p.Close()

	connections := []sqlui.Connection{
		{
			ID:       "demo",
			Name:     "Demo SQLite",
			Type:     "SQLite",
			Provider: p,
			Enabled:  true,
		},
	}

	if dsn := postgresDSN(); dsn == "" {
		log.Println("postgres: POSTGRES_HOST not set (datacenterDB disabled)")
	} else {
		pg, err := openPostgres("datacenterDB", dsn)
		if err != nil {
			log.Printf("open postgres: %v (datacenterDB disabled)", err)
		} else {
			defer pg.Close()
			connections = append(connections, sqlui.Connection{
				ID:       "datacenterDB",
				Name:     "datacenterDB",
				Type:     "PostgreSQL",
				Provider: pg,
				Enabled:  true,
			})
			log.Println("registered datacenterDB (PostgreSQL, connection from POSTGRES_* env)")
		}
	}

	mux := http.NewServeMux()
	err = sqlui.Register(mux, sqlui.Config{
		Prefix:      "/sqlui",
		Version:     "example-1.0.0",
		Connections: connections,
	})
	if err != nil {
		log.Fatalf("register sqlui: %v", err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sqlui/", http.StatusFound)
	})

	log.Println("SQL UI available at http://localhost" + addr + "/sqlui/")
	log.Fatal(http.ListenAndServe(addr, mux))
}
