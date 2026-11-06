package sqlui

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

type fakeProvider struct {
	dbs       []string
	tables    []string
	cols      []ColumnInfo
	queryRows *sql.Rows
	tableRows *sql.Rows
}

func (p *fakeProvider) GetDatabases(context.Context) ([]string, error) { return p.dbs, nil }
func (p *fakeProvider) GetTables(context.Context, string) ([]string, error) {
	return p.tables, nil
}
func (p *fakeProvider) GetColumns(context.Context, string, string) ([]ColumnInfo, error) {
	return p.cols, nil
}
func (p *fakeProvider) ExecuteSQL(context.Context, string) (*sql.Rows, error) {
	return p.queryRows, nil
}
func (p *fakeProvider) SelectTable(context.Context, string, string, int) (*sql.Rows, error) {
	return p.tableRows, nil
}

type fakeExecProvider struct {
	fakeProvider
	result ExecResult
}

func (p *fakeExecProvider) ExecSQL(context.Context, string) (ExecResult, error) {
	return p.result, nil
}

type fakeEditor struct {
	fakeProvider
	keys     []string
	updated  bool
	inserted bool
	deleted  bool
}

func (p *fakeEditor) PrimaryKeys(context.Context, string, string) ([]string, error) {
	return p.keys, nil
}
func (p *fakeEditor) UpdateRow(context.Context, string, string, map[string]any, map[string]any) (int64, error) {
	p.updated = true
	return 1, nil
}
func (p *fakeEditor) InsertRow(context.Context, string, string, map[string]any) (int64, error) {
	p.inserted = true
	return 42, nil
}
func (p *fakeEditor) DeleteRow(context.Context, string, string, map[string]any) (int64, error) {
	p.deleted = true
	return 1, nil
}

func mustRows(t *testing.T, rs *sqlmock.Rows) *sql.Rows {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	mock.ExpectQuery(".*").WillReturnRows(rs)
	rows, err := db.Query("SELECT")
	if err != nil {
		t.Fatalf("db.Query: %v", err)
	}
	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
	})
	return rows
}

func newMux(t *testing.T, p Provider, maxRows int, enabled bool) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	cfg := Config{
		Prefix: "/r",
		Connections: []Connection{{
			ID: "c1", Name: "c1", Type: "test", Provider: p, Enabled: enabled,
		}},
	}
	if maxRows > 0 {
		cfg.MaxRows = maxRows
	}
	if err := Register(mux, cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return mux
}

func request(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("unmarshal %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestConnectionsAndMetadata(t *testing.T) {
	p := &fakeProvider{
		dbs:    []string{"main"},
		tables: []string{"users", "orders"},
		cols:   []ColumnInfo{{Name: "id", Type: "INTEGER", Key: "PRI"}},
	}
	mux := newMux(t, p, 0, true)

	conns := decode[[]connectionDTO](t, request(t, mux, http.MethodGet, "/r/api/connections", ""))
	if len(conns) != 1 || conns[0].ID != "c1" || !conns[0].Capabilities.Query {
		t.Fatalf("connections = %+v", conns)
	}
	if conns[0].Capabilities.Exec || conns[0].Capabilities.Editing {
		t.Fatalf("unexpected optional capabilities: %+v", conns[0].Capabilities)
	}

	dbs := decode[[]string](t, request(t, mux, http.MethodGet, "/r/api/sql/c1/databases", ""))
	if len(dbs) != 1 || dbs[0] != "main" {
		t.Fatalf("databases = %v", dbs)
	}

	tables := decode[[]string](t, request(t, mux, http.MethodGet, "/r/api/sql/c1/tables?database=main", ""))
	if len(tables) != 2 || tables[1] != "orders" {
		t.Fatalf("tables = %v", tables)
	}

	cols := decode[[]ColumnInfo](t, request(t, mux, http.MethodGet, "/r/api/sql/c1/columns?database=main&table=users", ""))
	if len(cols) != 1 || cols[0].Name != "id" || cols[0].Key != "PRI" {
		t.Fatalf("columns = %+v", cols)
	}
}

func TestQuerySerializesRows(t *testing.T) {
	rows := mustRows(t, sqlmock.NewRows([]string{"id", "name", "note"}).
		AddRow(1, "alice", nil).
		AddRow(2, "bob", "hello"))
	p := &fakeProvider{queryRows: rows}
	mux := newMux(t, p, 0, true)

	rec := request(t, mux, http.MethodPost, "/r/api/sql/c1/query", `{"sql":"SELECT * FROM users"}`)
	res := decode[queryResponse](t, rec)
	if res.Error != nil {
		t.Fatalf("query error: %s", *res.Error)
	}
	if res.RowCount != 2 || len(res.Rows) != 2 || res.Rows[1][2] != "hello" {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Columns) != 3 || res.Columns[0].Name != "id" {
		t.Fatalf("columns = %+v", res.Columns)
	}
}

func TestQueryExec(t *testing.T) {
	p := &fakeExecProvider{result: ExecResult{RowsAffected: 3, LastInsertID: 7}}
	mux := newMux(t, p, 0, true)

	res := decode[queryResponse](t, request(t, mux, http.MethodPost, "/r/api/sql/c1/query",
		`{"sql":"UPDATE users SET name='x'"}`))
	if res.Error != nil {
		t.Fatalf("exec error: %s", *res.Error)
	}
	if res.RowsAffected == nil || *res.RowsAffected != 3 {
		t.Fatalf("rowsAffected = %v", res.RowsAffected)
	}
	if res.LastInsertID == nil || *res.LastInsertID != 7 {
		t.Fatalf("lastInsertId = %v", res.LastInsertID)
	}
}

func TestQueryReadOnlyProvider(t *testing.T) {
	mux := newMux(t, &fakeProvider{}, 0, true)
	rec := request(t, mux, http.MethodPost, "/r/api/sql/c1/query", `{"sql":"DELETE FROM users"}`)
	var env map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(env["error"], "read-only") {
		t.Fatalf("error = %q, want read-only", env["error"])
	}
}

func TestTableDataTruncated(t *testing.T) {
	rows := mustRows(t, sqlmock.NewRows([]string{"id"}).
		AddRow(1).AddRow(2).AddRow(3))
	p := &fakeProvider{tableRows: rows}
	mux := newMux(t, p, 2, true)

	res := decode[queryResponse](t, request(t, mux, http.MethodGet, "/r/api/sql/c1/table-data?database=main&table=users", ""))
	if res.RowCount != 2 || !res.Truncated || len(res.Rows) != 2 {
		t.Fatalf("result = %+v", res)
	}
}

func TestDisabledConnection404(t *testing.T) {
	mux := newMux(t, &fakeProvider{}, 0, false)
	rec := request(t, mux, http.MethodGet, "/r/api/sql/c1/databases", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestRowEditing(t *testing.T) {
	p := &fakeEditor{keys: []string{"id"}}
	mux := newMux(t, p, 0, true)

	keys := decode[[]string](t, request(t, mux, http.MethodGet, "/r/api/sql/c1/primary-keys?database=main&table=users", ""))
	if len(keys) != 1 || keys[0] != "id" {
		t.Fatalf("primary keys = %v", keys)
	}

	res := decode[map[string]any](t, request(t, mux, http.MethodPost, "/r/api/sql/c1/row",
		`{"action":"update","database":"main","table":"users","pk":{"id":1},"changes":{"name":"x"}}`))
	if res["rowsAffected"] != float64(1) || !p.updated {
		t.Fatalf("update result = %+v, updated=%v", res, p.updated)
	}

	res = decode[map[string]any](t, request(t, mux, http.MethodPost, "/r/api/sql/c1/row",
		`{"action":"insert","database":"main","table":"users","values":{"name":"y"}}`))
	if res["lastInsertId"] != float64(42) || !p.inserted {
		t.Fatalf("insert result = %+v, inserted=%v", res, p.inserted)
	}

	res = decode[map[string]any](t, request(t, mux, http.MethodPost, "/r/api/sql/c1/row",
		`{"action":"delete","database":"main","table":"users","pk":{"id":1}}`))
	if res["rowsAffected"] != float64(1) || !p.deleted {
		t.Fatalf("delete result = %+v, deleted=%v", res, p.deleted)
	}
}
