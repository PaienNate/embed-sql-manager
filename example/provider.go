package main

import (
	"context"
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"embedSqlClient/sqlui"
)

// sqliteProvider is a reference host-side Provider/ExecProvider/RowEditor
// implementation. It uses only database/sql + the SQLite driver; the sqlui
// package itself has no driver dependency.
type sqliteProvider struct {
	db *sql.DB
}

func openSQLite(path string) (*sqliteProvider, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// SQLite is single-writer; one connection keeps the demo predictable.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	p := &sqliteProvider{db: db}
	if err := p.seed(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return p, nil
}

func (p *sqliteProvider) Close() error { return p.db.Close() }

func (p *sqliteProvider) seed() error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL,
			note TEXT
		)`,
	} {
		if _, err := p.db.Exec(stmt); err != nil {
			return err
		}
	}
	var n int
	if err := p.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	for _, stmt := range []string{
		`INSERT INTO users (name,email) VALUES ('Alice','alice@example.com')`,
		`INSERT INTO users (name,email) VALUES ('Bob','bob@example.com')`,
		`INSERT INTO users (name,email) VALUES ('Carol',NULL)`,
		`INSERT INTO orders (user_id,amount,note) VALUES (1, 29.90, '鼠标')`,
		`INSERT INTO orders (user_id,amount,note) VALUES (1, 99.00, '键盘')`,
		`INSERT INTO orders (user_id,amount,note) VALUES (2, 9.90, NULL)`,
	} {
		if _, err := p.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (p *sqliteProvider) GetDatabases(context.Context) ([]string, error) {
	return []string{"main"}, nil
}

func (p *sqliteProvider) GetTables(ctx context.Context, _ string) ([]string, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (p *sqliteProvider) GetColumns(ctx context.Context, _, table string) ([]sqlui.ColumnInfo, error) {
	rows, err := p.db.QueryContext(ctx, "PRAGMA table_info("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var cols []sqlui.ColumnInfo
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		col := sqlui.ColumnInfo{
			Name:     name,
			Type:     typ,
			Nullable: notNull == 0,
			Default:  dflt.String,
		}
		if pk > 0 {
			col.Key = "PRI"
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func (p *sqliteProvider) ExecuteSQL(ctx context.Context, sqlText string) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, sqlText)
}

func (p *sqliteProvider) SelectTable(ctx context.Context, _, table string, limit int) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, "SELECT * FROM "+quoteIdent(table)+" LIMIT ?", limit)
}

func (p *sqliteProvider) ExecSQL(ctx context.Context, sqlText string) (sqlui.ExecResult, error) {
	res, err := p.db.ExecContext(ctx, sqlText)
	if err != nil {
		return sqlui.ExecResult{}, err
	}
	n, _ := res.RowsAffected()
	id, _ := res.LastInsertId()
	return sqlui.ExecResult{RowsAffected: n, LastInsertID: id}, nil
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
