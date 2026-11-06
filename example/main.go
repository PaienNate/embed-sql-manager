package main

import (
	"log"
	"net/http"
	"os"

	"embedSqlClient/sqlui"
)

func main() {
	dbPath := os.Getenv("SQLUI_DB")
	if dbPath == "" {
		dbPath = "demo.db"
	}
	p, err := openSQLite(dbPath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer p.Close()

	mux := http.NewServeMux()
	err = sqlui.Register(mux, sqlui.Config{
		Prefix:  "/sqlui",
		Version: "example-1.0.0",
		Connections: []sqlui.Connection{
			{
				ID:       "demo",
				Name:     "Demo SQLite",
				Type:     "SQLite",
				Provider: p,
				Enabled:  true,
			},
		},
	})
	if err != nil {
		log.Fatalf("register sqlui: %v", err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sqlui/", http.StatusFound)
	})

	log.Println("SQL UI available at http://localhost:8081/sqlui/")
	log.Fatal(http.ListenAndServe(":8081", mux))
}
