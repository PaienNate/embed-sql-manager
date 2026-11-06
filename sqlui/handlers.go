package sqlui

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type connectionDTO struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Type         string       `json:"type"`
	Enabled      bool         `json:"enabled"`
	Capabilities Capabilities `json:"capabilities"`
}

func (s *server) connectionFor(w http.ResponseWriter, r *http.Request) (Connection, bool) {
	c, ok := s.conns[r.PathValue("connID")]
	if !ok || !c.Enabled {
		http.Error(w, "sqlui: connection not found", http.StatusNotFound)
		return Connection{}, false
	}
	return c, true
}

func (s *server) requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), s.queryTimeout)
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, map[string]string{"error": err.Error()})
}

func (s *server) handleDatabases(w http.ResponseWriter, r *http.Request) {
	c, ok := s.connectionFor(w, r)
	if !ok {
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	dbs, err := c.Provider.GetDatabases(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	if dbs == nil {
		dbs = []string{}
	}
	writeJSON(w, dbs)
}

func (s *server) handleTables(w http.ResponseWriter, r *http.Request) {
	c, ok := s.connectionFor(w, r)
	if !ok {
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	tables, err := c.Provider.GetTables(ctx, r.URL.Query().Get("database"))
	if err != nil {
		writeError(w, err)
		return
	}
	if tables == nil {
		tables = []string{}
	}
	writeJSON(w, tables)
}

func (s *server) handleColumns(w http.ResponseWriter, r *http.Request) {
	c, ok := s.connectionFor(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	table := q.Get("table")
	if strings.TrimSpace(table) == "" {
		writeError(w, &badRequest{"table is required"})
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	cols, err := c.Provider.GetColumns(ctx, q.Get("database"), table)
	if err != nil {
		writeError(w, err)
		return
	}
	if cols == nil {
		cols = []ColumnInfo{}
	}
	writeJSON(w, cols)
}

func (s *server) handleTableData(w http.ResponseWriter, r *http.Request) {
	c, ok := s.connectionFor(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	table := q.Get("table")
	if strings.TrimSpace(table) == "" {
		writeError(w, &badRequest{"table is required"})
		return
	}
	limit := s.maxRows
	if raw := q.Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > s.maxRows {
		limit = s.maxRows
	}

	ctx, cancel := s.requestContext(r)
	defer cancel()
	start := time.Now()
	rows, err := c.Provider.SelectTable(ctx, q.Get("database"), table, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	defer func() { _ = rows.Close() }()
	res, err := readRows(rows, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	res.ElapsedMS = time.Since(start).Milliseconds()
	writeJSON(w, res)
}

type queryRequest struct {
	SQL string `json:"sql"`
}

func (s *server) handleQuery(w http.ResponseWriter, r *http.Request) {
	c, ok := s.connectionFor(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &badRequest{"invalid request body"})
		return
	}
	sqlText := strings.TrimSpace(req.SQL)
	if sqlText == "" {
		writeError(w, &badRequest{"sql is required"})
		return
	}

	ctx, cancel := s.requestContext(r)
	defer cancel()
	start := time.Now()

	if isQuerySQL(sqlText) {
		rows, err := c.Provider.ExecuteSQL(ctx, sqlText)
		if err != nil {
			writeError(w, err)
			return
		}
		defer func() { _ = rows.Close() }()
		res, err := readRows(rows, s.maxRows)
		if err != nil {
			writeError(w, err)
			return
		}
		res.ElapsedMS = time.Since(start).Milliseconds()
		writeJSON(w, res)
		return
	}

	ep, ok := c.Provider.(ExecProvider)
	if !ok {
		writeError(w, &badRequest{"this provider is read-only (ExecProvider is not implemented)"})
		return
	}
	result, err := ep.ExecSQL(ctx, sqlText)
	if err != nil {
		writeError(w, err)
		return
	}
	affected, lastID := result.RowsAffected, result.LastInsertID
	writeJSON(w, &queryResponse{
		ElapsedMS:    time.Since(start).Milliseconds(),
		RowsAffected: &affected,
		LastInsertID: &lastID,
	})
}

type badRequest struct{ msg string }

func (e *badRequest) Error() string { return e.msg }
