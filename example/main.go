package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"embedSqlClient/sqlui"
)

// Demo-only datacenterDB connection. The credentials are hardcoded in this
// example on purpose; do not reuse them outside this local demo.
const (
	pgHost     = "YOUR_POSTGRES_HOST"
	pgPort     = 5432
	pgUser     = "postgres"
	pgPassword = "YOUR_POSTGRES_PASSWORD"
	pgDatabase = "YOUR_POSTGRES_DBNAME"
	pgConfig   = "sslmode=disable search_path=YOUR_POSTGRES_SCHEMA TimeZone=Asia/Shanghai connect_timeout=10"
)

func postgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s %s",
		pgHost, pgPort, pgUser, pgPassword, pgDatabase, pgConfig,
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

	pg, err := openPostgres("datacenterDB", postgresDSN())
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
		log.Printf("registered datacenterDB (%s/%s, search_path from config)", pgHost, pgDatabase)
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
