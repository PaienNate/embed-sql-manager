// Package sqlui embeds a minimal SQL console UI into an arbitrary Go program.
//
// The host implements Provider (and optionally ExecProvider / RowEditor) with
// whatever database stack it already uses — database/sql, sqlx, sqlex, GORM,
// ... — and registers the connections. This package itself only imports the
// Go standard library: it never imports a database driver or an ORM.
package sqlui

import (
	"context"
	"database/sql"
)

// Provider is the database-specific surface the UI needs.
//
// ExecuteSQL is the raw SQL console path (SELECT/WITH/SHOW/DESCRIBE/EXPLAIN/
// PRAGMA/...). SelectTable is the table-preview path; the provider is
// responsible for quoting the identifier and applying the limit in its own
// dialect.
type Provider interface {
	GetDatabases(ctx context.Context) ([]string, error)
	GetTables(ctx context.Context, database string) ([]string, error)
	GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error)

	ExecuteSQL(ctx context.Context, sql string) (*sql.Rows, error)
	SelectTable(ctx context.Context, database, table string, limit int) (*sql.Rows, error)
}

// ExecProvider is optional: it enables INSERT/UPDATE/DELETE/DDL from the SQL
// console. Without it the console is read-only. Row-level editing is not part
// of this library on purpose; users write INSERT/UPDATE/DELETE in the console.
type ExecProvider interface {
	ExecSQL(ctx context.Context, sql string) (ExecResult, error)
}

// Connection is one host-registered data source.
type Connection struct {
	ID       string
	Name     string
	Type     string // free-form display label: SQLite / PostgreSQL / TDengine / ...
	Provider Provider
	Enabled  bool
}

// ColumnInfo describes one result/table column. The fields beyond Name/Type
// are filled when the provider knows them; the core also fills Name/Type/
// Nullable/Default from database/sql's ColumnTypes for query results.
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
	Key      string `json:"key,omitempty"`
	Comment  string `json:"comment,omitempty"`
	Binary   bool   `json:"binary,omitempty"`
}

// ExecResult is returned by ExecProvider.ExecSQL.
type ExecResult struct {
	RowsAffected int64 `json:"rowsAffected"`
	LastInsertID int64 `json:"lastInsertId"`
}

// Capabilities tells the UI which optional interfaces the provider implements.
type Capabilities struct {
	Query bool `json:"query"`
	Exec  bool `json:"exec"`
}

func capabilitiesOf(p Provider) Capabilities {
	_, exec := p.(ExecProvider)
	return Capabilities{Query: true, Exec: exec}
}
