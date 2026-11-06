package sqlui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Version is the default version reported to the frontend when Config.Version
// is empty. Hosts can override it per Register call.
const Version = "0.1.0"

// Config configures a registered SQL UI instance.
type Config struct {
	Prefix      string
	Connections []Connection
	Middleware  func(http.Handler) http.Handler
	// Version is shown in the UI. When empty, Version is used.
	Version string
	// MaxRows caps the rows returned by a query or a table preview. Default 1000.
	MaxRows int
	// QueryTimeout bounds one query. Default 30s.
	QueryTimeout time.Duration
}

type server struct {
	conns        map[string]Connection
	order        []string
	prefix       string
	version      string
	frontend     fs.FS
	middle       func(http.Handler) http.Handler
	maxRows      int
	queryTimeout time.Duration
}

// Register mounts the SQL UI handlers under cfg.Prefix on mux. Prefix defaults
// to "/sqlui". It returns an error for an invalid prefix, a duplicate/empty
// connection ID, or a nil provider; nothing is mounted in that case.
func Register(mux *http.ServeMux, cfg Config) error {
	if mux == nil {
		return errors.New("sqlui: Register: nil mux")
	}

	prefix, err := normalizePrefix(cfg.Prefix)
	if err != nil {
		return err
	}
	frontend, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		return fmt.Errorf("sqlui: embed frontend sub: %w", err)
	}
	version := cfg.Version
	if version == "" {
		version = Version
	}
	maxRows := cfg.MaxRows
	if maxRows <= 0 {
		maxRows = 1000
	}
	queryTimeout := cfg.QueryTimeout
	if queryTimeout <= 0 {
		queryTimeout = 30 * time.Second
	}

	s := &server{
		conns:        map[string]Connection{},
		prefix:       prefix,
		version:      version,
		frontend:     frontend,
		middle:       cfg.Middleware,
		maxRows:      maxRows,
		queryTimeout: queryTimeout,
	}
	for _, c := range cfg.Connections {
		if strings.TrimSpace(c.ID) == "" {
			return errors.New("sqlui: Register: connection ID must not be empty")
		}
		if c.Provider == nil {
			return fmt.Errorf("sqlui: Register: connection %q has a nil Provider", c.ID)
		}
		if _, dup := s.conns[c.ID]; dup {
			return fmt.Errorf("sqlui: Register: duplicate connection ID %q", c.ID)
		}
		s.conns[c.ID] = c
		s.order = append(s.order, c.ID)
	}

	// Every route is wrapped so Config.Middleware covers the whole API and the
	// SPA exactly like the Redis UI's Register.
	mux.Handle("GET "+prefix+"/api/connections", s.wrap(http.HandlerFunc(s.handleConnections)))
	mux.Handle("GET "+prefix+"/api/sql/{connID}/databases", s.wrap(http.HandlerFunc(s.handleDatabases)))
	mux.Handle("GET "+prefix+"/api/sql/{connID}/tables", s.wrap(http.HandlerFunc(s.handleTables)))
	mux.Handle("GET "+prefix+"/api/sql/{connID}/columns", s.wrap(http.HandlerFunc(s.handleColumns)))
	mux.Handle("GET "+prefix+"/api/sql/{connID}/table-data", s.wrap(http.HandlerFunc(s.handleTableData)))
	mux.Handle("POST "+prefix+"/api/sql/{connID}/query", s.wrap(http.HandlerFunc(s.handleQuery)))

	// SPA catch-all. It is less specific than the API patterns above, so API
	// requests are unaffected.
	mux.Handle(prefix+"/", s.wrap(http.HandlerFunc(s.serveFrontend)))
	return nil
}

func (s *server) wrap(h http.Handler) http.Handler {
	if s.middle != nil {
		return s.middle(h)
	}
	return h
}

func (s *server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, s.prefix)
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		name = "index.html"
	}
	if _, err := fs.Stat(s.frontend, name); err != nil {
		name = "index.html"
	}
	if name == "index.html" {
		s.serveIndexHTML(w)
		return
	}
	http.ServeFileFS(w, r, s.frontend, name)
}

func (s *server) serveIndexHTML(w http.ResponseWriter) {
	data, err := fs.ReadFile(s.frontend, "index.html")
	if err != nil {
		http.Error(w, "sqlui: frontend index not found", http.StatusInternalServerError)
		return
	}
	script := `<script>window.__SQLUI__={apiBase:"` + s.prefix + `/api",version:` + strconv.Quote(s.version) + `}</script>`
	html := strings.Replace(string(data), "</head>", script+"</head>", 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(html))
}

func (s *server) handleConnections(w http.ResponseWriter, _ *http.Request) {
	out := make([]connectionDTO, 0, len(s.order))
	for _, id := range s.order {
		c := s.conns[id]
		if !c.Enabled {
			continue
		}
		out = append(out, connectionDTO{
			ID:           c.ID,
			Name:         c.Name,
			Type:         c.Type,
			Enabled:      true,
			Capabilities: capabilitiesOf(c.Provider),
		})
	}
	writeJSON(w, out)
}

// normalizePrefix trims and normalizes cfg.Prefix into the mount prefix. An
// empty prefix keeps the documented "/sqlui" default. A prefix that resolves
// to the mux root is rejected because the SPA catch-all would swallow every
// host route.
func normalizePrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "/sqlui", nil
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "", errors.New(`sqlui: Register: prefix "/" would mount the UI at the mux root`)
	}
	return prefix, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
