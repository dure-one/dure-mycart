package database

// sqliteDialect targets the embedded database. Every method here renders the
// SQL myCart has always sent to SQLite, so the SQLite code path is unchanged by
// the introduction of a second dialect.
type sqliteDialect struct{}

func (sqliteDialect) Name() string         { return DriverSQLite }
func (sqliteDialect) Driver() string       { return "sqlite" }
func (sqliteDialect) GooseDialect() string { return "sqlite3" }

// Rebind is the identity: the queries are already written with `?`.
func (sqliteDialect) Rebind(query string) string { return query }

func (sqliteDialect) Epoch(column string) string { return "strftime('%s', " + column + ")" }

func (sqliteDialect) JSONObject(pairs string) string { return "json_object(" + pairs + ")" }

func (sqliteDialect) JSONAgg(inner string) string  { return "json_group_array(" + inner + ")" }
func (sqliteDialect) JSONValue(expr string) string { return "json(" + expr + ")" }

func (sqliteDialect) JSONBool(column string) string {
	return "CASE WHEN " + column + " = 1 THEN json('true') ELSE json('false') END"
}
