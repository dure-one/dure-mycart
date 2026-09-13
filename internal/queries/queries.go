package queries

import (
	"io/fs"
	"sync/atomic"

	"github.com/shurco/mycart/internal/database"
)

// db holds the process-wide handle. It is an atomic pointer rather than a
// plain variable because the installer replaces it at runtime, after the
// operator picks a different database, while requests are already being served.
var db atomic.Pointer[Base]

// Base aggregates the query groups for settings, authentication, installation,
// pages, products, carts and storefront customers.
//
// Every group shares one handle, so the whole application always talks to the
// same database in the same dialect.
type Base struct {
	conn *database.Conn

	SettingQueries
	AuthQueries
	InstallQueries
	PageQueries
	ProductQueries
	CartQueries
	CustomerQueries
}

// NewBase wires every query group to conn.
func NewBase(conn *database.Conn) *Base {
	return &Base{
		conn:            conn,
		AuthQueries:     AuthQueries{DB: conn},
		InstallQueries:  InstallQueries{DB: conn},
		SettingQueries:  SettingQueries{DB: conn},
		PageQueries:     PageQueries{DB: conn},
		ProductQueries:  ProductQueries{DB: conn},
		CartQueries:     CartQueries{DB: conn},
		CustomerQueries: CustomerQueries{DB: conn},
	}
}

// New opens the configured database, brings the schema up to date and installs
// it as the process-wide handle.
func New(cfg database.Config, migrations fs.FS) error {
	conn, err := database.Open(cfg, migrations)
	if err != nil {
		return err
	}

	SetConn(conn)
	database.SetActive(cfg)
	return nil
}

// SetConn installs conn as the process-wide handle.
func SetConn(conn *database.Conn) {
	db.Store(NewBase(conn))
}

// Swap installs conn as the process-wide handle and returns the handle it
// replaced, so the caller can close it once it is sure the switch worked.
func Swap(conn *database.Conn) (old *database.Conn) {
	old = Conn()
	db.Store(NewBase(conn))
	return old
}

// Conn returns the handle every query group shares, or nil if the database has
// not been initialised yet. Use New or SetConn first.
func Conn() *database.Conn {
	if b := db.Load(); b != nil {
		return b.conn
	}
	return nil
}

// DB returns the Base instance. If the database is not initialized, returns nil.
// Use New() to initialize the database before calling DB().
func DB() *Base {
	return db.Load()
}
