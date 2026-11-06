package sqlui

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// queryResponse is the JSON envelope for /query and /table-data.
type queryResponse struct {
	Error        *string      `json:"error"`
	Columns      []ColumnInfo `json:"columns,omitempty"`
	Rows         [][]any      `json:"rows,omitempty"`
	RowCount     int          `json:"rowCount"`
	Truncated    bool         `json:"truncated,omitempty"`
	ElapsedMS    int64        `json:"elapsedMs"`
	RowsAffected *int64       `json:"rowsAffected,omitempty"`
	LastInsertID *int64       `json:"lastInsertId,omitempty"`
}

// readRows serializes at most maxRows rows. The caller owns rows and must
// close it.
func readRows(rows *sql.Rows, maxRows int) (*queryResponse, error) {
	if rows == nil {
		return &queryResponse{Columns: []ColumnInfo{}, Rows: [][]any{}}, nil
	}
	names, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read columns: %w", err)
	}
	types, _ := rows.ColumnTypes()

	cols := make([]ColumnInfo, len(names))
	for i, name := range names {
		info := ColumnInfo{Name: name}
		if i < len(types) && types[i] != nil {
			info.Type = types[i].DatabaseTypeName()
			if info.Type == "" {
				if st := types[i].ScanType(); st != nil {
					info.Type = fmt.Sprint(st)
				}
			}
			if nullable, ok := types[i].Nullable(); ok {
				info.Nullable = nullable
			}
			info.Binary = isBinaryType(info.Type)
		}
		cols[i] = info
	}

	res := &queryResponse{Columns: cols, Rows: [][]any{}}
	count := 0
	for rows.Next() {
		if count >= maxRows {
			// We know there is at least one more row because Next() succeeded.
			res.Truncated = true
			break
		}
		values := make([]any, len(names))
		ptrs := make([]any, len(names))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		out := make([]any, len(values))
		for i, v := range values {
			out[i] = normalizeValue(v, cols[i].Binary)
		}
		res.Rows = append(res.Rows, out)
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}
	res.RowCount = count
	return res, nil
}

// isBinaryType is a best-effort type-name check. database/sql's ColumnTypes
// database type names vary by driver, so this only marks the common BLOB-ish
// families; non-UTF-8 []byte values are still base64-encoded below.
func isBinaryType(t string) bool {
	t = strings.ToUpper(t)
	switch {
	case strings.Contains(t, "BLOB"),
		strings.Contains(t, "BYTEA"),
		strings.Contains(t, "VARBINARY"),
		strings.Contains(t, "BINARY"),
		strings.Contains(t, "IMAGE"),
		t == "RAW":
		return true
	}
	return false
}

func normalizeValue(v any, binary bool) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case []byte:
		if binary || !utf8.Valid(x) {
			return base64.StdEncoding.EncodeToString(x)
		}
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case int64, float64, bool, string:
		return x
	default:
		return fmt.Sprint(x)
	}
}
