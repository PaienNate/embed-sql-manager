package sqlui

import "strings"

// isQuerySQL is a deliberately small classifier: the first keyword decides
// whether the console calls Provider.ExecuteSQL or ExecProvider.ExecSQL. It is
// not a full parser; writable CTEs (WITH ... INSERT/UPDATE) and dialect
// extensions are the provider's responsibility.
func isQuerySQL(sql string) bool {
	s := stripLeadingSQLComments(strings.TrimSpace(sql))
	if s == "" {
		return false
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToUpper(fields[0]) {
	case "SELECT", "WITH", "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "PRAGMA", "VALUES", "TABLE", "CALL":
		return true
	default:
		return false
	}
}

func stripLeadingSQLComments(s string) string {
	for {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "--"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			return ""
		case strings.HasPrefix(s, "/*"):
			if i := strings.Index(s, "*/"); i >= 0 {
				s = s[i+2:]
				continue
			}
			return ""
		default:
			return s
		}
	}
}
