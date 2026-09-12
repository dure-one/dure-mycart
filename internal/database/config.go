package database

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

const (
	// DirBase is the runtime-writable directory holding the SQLite database and
	// the database configuration file.
	DirBase = "./lc_base"
	// ConfigPath is where the installer records the chosen database.
	ConfigPath = "./lc_base/config.json"
	// DefaultSQLiteDSN is the embedded database used when nothing is
	// configured — the behaviour myCart has always had.
	DefaultSQLiteDSN = "./lc_base/data.db"
)

// Environment variables read during configuration resolution.
const (
	EnvDriver = "MYCART_DB_DRIVER"
	EnvDSN    = "MYCART_DB_DSN"
)

// Configuration sources, in descending priority.
const (
	SourceFlag    = "flag"
	SourceEnv     = "env"
	SourceFile    = "file"
	SourceDefault = "default"
	// SourceWizard is used for a database picked in the install wizard. It is
	// recorded so the choice can be told apart from the built-in default, but
	// like the default it does not pin the installation.
	SourceWizard = "wizard"
)

// Config is the resolved database configuration.
type Config struct {
	Driver string
	DSN    string
	// Source records where the values came from, so the install wizard can
	// tell an operator-chosen database from the built-in default.
	Source string
}

// Pinned reports whether the database was chosen outside the application. A
// pinned configuration cannot be changed from the install wizard: the operator
// has already decided, and silently connecting somewhere else would be wrong.
func (c Config) Pinned() bool {
	return c.Source == SourceFlag || c.Source == SourceEnv
}

// redacted replaces every secret in a DSN for logs and API responses.
const redacted = "***"

// Redacted renders the DSN for logs and API responses with the password
// removed. Never log Config.DSN directly.
//
// pgx accepts a password in three places — the URL userinfo, the URL query
// string and the key=value form — and all three are masked here.
func (c Config) Redacted() string {
	if c.Driver != DriverPostgres {
		return c.DSN
	}

	if u, err := url.Parse(c.DSN); err == nil && u.Scheme != "" {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), redacted)
		}
		q := u.Query()
		for _, key := range sensitiveDSNKeys {
			if q.Has(key) {
				q.Set(key, redacted)
			}
		}
		u.RawQuery = q.Encode()
		// url.Values.Encode escapes `*`, so the mask comes back as %2A%2A%2A:
		// one %2A is one asterisk.
		return strings.ReplaceAll(u.String(), "%2A", "*")
	}

	// key=value form: password=... user=...
	parts := strings.Fields(c.DSN)
	for i, p := range parts {
		key, _, ok := strings.Cut(p, "=")
		if ok && isSensitiveDSNKey(key) {
			parts[i] = key + "=" + redacted
		}
	}
	return strings.Join(parts, " ")
}

// sensitiveDSNKeys are the connection options whose value is a secret.
var sensitiveDSNKeys = []string{"password", "sslpassword"}

func isSensitiveDSNKey(key string) bool {
	for _, sensitive := range sensitiveDSNKeys {
		if strings.EqualFold(key, sensitive) {
			return true
		}
	}
	return false
}

// Overrides carries values supplied on the command line. Empty fields mean
// "not set" and fall through to the next source.
type Overrides struct {
	Driver string
	DSN    string
}

// Resolve determines the database configuration from, in order of decreasing
// priority: command-line flags, environment variables, lc_base/config.json, and
// finally the embedded SQLite default. SQLite is the default so that an upgrade
// never changes where an existing installation stores its data.
func Resolve(o Overrides) (Config, error) {
	return ResolveFile(o, ConfigPath)
}

// ResolveFile is Resolve with an explicit configuration file path.
func ResolveFile(o Overrides, path string) (Config, error) {
	cfg := Config{Driver: DriverSQLite, DSN: DefaultSQLiteDSN, Source: SourceDefault}

	if fromFile, ok, err := readConfigFile(path); err != nil {
		return Config{}, err
	} else if ok {
		cfg = fromFile
	}

	if v := os.Getenv(EnvDriver); v != "" {
		cfg.Driver, cfg.Source = v, SourceEnv
	}
	if v := os.Getenv(EnvDSN); v != "" {
		cfg.DSN, cfg.Source = v, SourceEnv
	}

	if o.Driver != "" {
		cfg.Driver, cfg.Source = o.Driver, SourceFlag
	}
	if o.DSN != "" {
		cfg.DSN, cfg.Source = o.DSN, SourceFlag
	}

	return normalize(cfg)
}

// normalize validates the resolved configuration and repairs the one mistake
// that would otherwise fail in a confusing way: a PostgreSQL DSN with the
// driver left at its SQLite default.
func normalize(cfg Config) (Config, error) {
	if cfg.Driver == DriverSQLite && looksLikePostgresDSN(cfg.DSN) {
		cfg.Driver = DriverPostgres
	}

	switch cfg.Driver {
	case DriverSQLite:
		if cfg.DSN == "" {
			cfg.DSN = DefaultSQLiteDSN
		}
		return cfg, nil
	case DriverPostgres:
		// A PostgreSQL driver with the SQLite default path is not a
		// configuration anybody means: it happens when --db postgres is given
		// without --db-dsn, or when config.json names a driver and no DSN.
		if cfg.DSN == "" || cfg.DSN == DefaultSQLiteDSN {
			return Config{}, fmt.Errorf(
				"driver %q requires a connection string (%s or --db-dsn)", DriverPostgres, EnvDSN)
		}
		return cfg, nil
	default:
		return Config{}, fmt.Errorf("unknown database driver %q (want %q or %q)",
			cfg.Driver, DriverSQLite, DriverPostgres)
	}
}

func looksLikePostgresDSN(dsn string) bool {
	for _, scheme := range []string{"postgres://", "postgresql://", "pgx://"} {
		if strings.HasPrefix(dsn, scheme) {
			return true
		}
	}
	return false
}

// active holds the configuration the process is running on. It lives here,
// beside the connection code, because the HTTP handlers that need it cannot
// import the root package — the root package already imports them.
var active atomic.Pointer[Config]

// SetActive records the configuration the process is running on, so responses
// can report it and the install wizard can tell a pinned database from a
// configurable one.
func SetActive(c Config) { active.Store(&c) }

// Active returns the configuration the process is running on. Before anything
// has been set it reports the built-in default, which is what an installation
// that nobody has configured uses.
func Active() Config {
	if c := active.Load(); c != nil {
		return *c
	}
	return Config{Driver: DriverSQLite, DSN: DefaultSQLiteDSN, Source: SourceDefault}
}
