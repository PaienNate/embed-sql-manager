package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"

	"embedSqlClient/sqlui"
)

// postgresProvider is a host-side Provider/ExecProvider for PostgreSQL.
//
// It lives in the example (not in sqlui) on purpose: the core library must not
// depend on a database driver. The metadata SQL is adapted from
// sys_api_service's PostgresQueryExecutor.
type postgresProvider struct {
	db *sql.DB
}

func openPostgres(label, dsn string) (*postgresProvider, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Do not fail the whole demo if the database is temporarily unreachable:
	// keep the connection registered and let the UI surface the error on demand.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Printf("postgres %s: ping failed (connection kept for retry): %v", label, err)
	} else {
		log.Printf("postgres %s: connected", label)
	}
	return &postgresProvider{db: db}, nil
}

func (p *postgresProvider) Close() error { return p.db.Close() }

// GetDatabases returns schemas, not cluster databases: the UI's "database"
// level maps naturally to a PostgreSQL schema. The current_schema() (the first
// existing entry of the DSN's search_path) is sorted first so the tree can
// auto-open it.
func (p *postgresProvider) GetDatabases(ctx context.Context) ([]string, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT schema_name
		  FROM information_schema.schemata
		 WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		   AND schema_name NOT LIKE 'pg_temp_%'
		   AND schema_name NOT LIKE 'pg_toast_temp_%'
		 ORDER BY CASE WHEN schema_name = current_schema() THEN 0 ELSE 1 END,
		          schema_name`)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan schema: %w", err)
		}
		schemas = append(schemas, name)
	}
	return schemas, rows.Err()
}

func (p *postgresProvider) GetTables(ctx context.Context, schema string) ([]string, error) {
	if schema == "" {
		schema = "public"
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT table_name
		  FROM information_schema.tables
		 WHERE table_schema = $1
		   AND table_type IN ('BASE TABLE', 'VIEW')
		 ORDER BY table_name`, schema)
	if err != nil {
		return nil, fmt.Errorf("list tables in %s: %w", schema, err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (p *postgresProvider) GetColumns(ctx context.Context, schema, table string) ([]sqlui.ColumnInfo, error) {
	if schema == "" {
		schema = "public"
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT a.attname,
		       pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
		       NOT a.attnotnull AS nullable,
		       COALESCE(pg_get_expr(d.adbin, d.adrelid), '') AS default_value,
		       COALESCE(col_description(a.attrelid, a.attnum), '') AS comment,
		       EXISTS (
		         SELECT 1
		           FROM pg_index i
		          WHERE i.indrelid = a.attrelid
		            AND i.indisprimary
		            AND a.attnum = ANY(i.indkey)
		       ) AS is_pk
		  FROM pg_attribute a
		  JOIN pg_class c ON a.attrelid = c.oid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		 WHERE c.relname = $2
		   AND n.nspname = $1
		   AND a.attnum > 0
		   AND NOT a.attisdropped
		 ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("list columns of %s.%s: %w", schema, table, err)
	}
	defer func() { _ = rows.Close() }()

	var columns []sqlui.ColumnInfo
	for rows.Next() {
		var (
			name, typ, dflt, comment string
			nullable, isPK           bool
		)
		if err := rows.Scan(&name, &typ, &nullable, &dflt, &comment, &isPK); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		col := sqlui.ColumnInfo{
			Name:     name,
			Type:     typ,
			Nullable: nullable,
			Default:  dflt,
			Comment:  comment,
		}
		if isPK {
			col.Key = "PRI"
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (p *postgresProvider) ExecuteSQL(ctx context.Context, sqlText string) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, sqlText)
}

func (p *postgresProvider) SelectTable(ctx context.Context, schema, table string, limit int) (*sql.Rows, error) {
	if schema == "" {
		schema = "public"
	}
	query := "SELECT * FROM " + quoteIdent(schema) + "." + quoteIdent(table) + " LIMIT $1"
	return p.db.QueryContext(ctx, query, limit)
}

func (p *postgresProvider) ExecSQL(ctx context.Context, sqlText string) (sqlui.ExecResult, error) {
	res, err := p.db.ExecContext(ctx, sqlText)
	if err != nil {
		return sqlui.ExecResult{}, err
	}
	affected, _ := res.RowsAffected()
	// PostgreSQL has no LastInsertId; use INSERT ... RETURNING in the console
	// if you need the generated key.
	return sqlui.ExecResult{RowsAffected: affected}, nil
}
