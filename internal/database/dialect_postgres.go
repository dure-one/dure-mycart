package database

// postgresDialect targets PostgreSQL. It is deliberately small: anything that
// could be expressed in both dialects is expressed once, in the shared SQL.
type postgresDialect struct{}

func (postgresDialect) Name() string         { return DriverPostgres }
func (postgresDialect) Driver() string       { return "pgx" }
func (postgresDialect) GooseDialect() string { return "postgres" }

func (postgresDialect) Rebind(query string) string { return rebindDollar(query) }

// Epoch must cast: EXTRACT(EPOCH FROM ...) returns numeric in PostgreSQL, and
// database/sql cannot scan numeric into an int64.
func (postgresDialect) Epoch(column string) string {
	return "EXTRACT(EPOCH FROM " + column + ")::bigint"
}

func (postgresDialect) JSONObject(pairs string) string { return "json_build_object(" + pairs + ")" }

func (postgresDialect) JSONAgg(inner string) string  { return "json_agg(" + inner + ")" }
func (postgresDialect) JSONValue(expr string) string { return "(" + expr + ")::json" }

// JSONBool is the identity: PostgreSQL booleans already serialise as JSON
// true/false.
func (postgresDialect) JSONBool(column string) string { return column }
