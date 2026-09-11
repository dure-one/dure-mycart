package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/webutil"
)

type installStatus struct {
	Installed bool `json:"installed"`
}

// InstallStatus reports whether first-time setup has been completed.
//
// @Summary      Installation status
// @Description  Returns whether the cart has been installed
// @Tags         Install
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse{result=installStatus}
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/install/status [get]
func InstallStatus(c fiber.Ctx) error {
	// Use package-level flag instead of querying database
	installed := !db.InstallRequired()

	return webutil.Response(c, fiber.StatusOK, "Installation status", installStatus{Installed: installed})
}

// Install performs the initial installation of the application.
//
// @Summary      Install application
// @Description  Perform initial setup with admin credentials and domain
// @Tags         Install
// @Accept       json
// @Produce      json
// @Param        request body models.Install true "Installation data"
// @Success      200 {object} webutil.HTTPResponse "Cart installed"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/install [post]
func Install(c fiber.Ctx) error {
	log := logging.New()
	request := new(models.Install)

	if err := c.Bind().Body(request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := request.Validate(); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	// Build database config from user's request
	dbConfig := buildConfigFromRequest(request)

	// Run migrations with user-selected config
	if err := db.MigrateWithConfig(dbConfig, migrations.Embed()); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Reinitialize store with migrated database
	store.InitStoreWithType(db.DB(), db.Type())

	// Create admin user and initial settings
	if err := store.Install(c.Context(), request); err != nil {
		if errors.Is(err, store.ErrAlreadyInstalled) {
			return webutil.StatusBadRequest(c, err.Error())
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Mark as installed, migrations now enabled
	db.SetInstalled()

	log.Info().Msg("Installation completed successfully")

	return webutil.Response(c, fiber.StatusOK, "Cart installed", nil)
}

// buildConfigFromRequest builds a database Config from the install request.
// Used to connect and migrate with user-selected database configuration.
func buildConfigFromRequest(req *models.Install) *db.Config {
	cfg := &db.Config{
		Type: req.DBType,
	}

	if req.DBType == "postgres" || req.DBType == "postgresql" {
		// Parse DATABASE_URL using existing parseConnectionURL logic
		defaults := db.PostgresConfig{
			Host:           "localhost",
			Port:           5432,
			Database:       "mycart",
			User:           "postgres",
			SSLMode:        "require",
			ConnectTimeout: 10,
			MaxOpenConns:   25,
			MaxIdleConns:   5,
		}
		cfg.PostgreSQL = parseConnectionURL(req.DatabaseURL, defaults)
	} else {
		cfg.SQLite = db.SQLiteConfig{
			Path: req.SQLitePath,
		}
	}

	return cfg
}

// parseConnectionURL parses a PostgreSQL connection URL.
func parseConnectionURL(rawURL string, defaults db.PostgresConfig) db.PostgresConfig {
	u, err := url.Parse(rawURL)
	if err != nil {
		fmt.Printf("⚠️  Failed to parse DATABASE_URL: %v\n", err)
		return defaults
	}

	cfg := defaults
	if u.Hostname() != "" {
		cfg.Host = u.Hostname()
	}
	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			cfg.Port = port
		}
	}
	if u.User != nil {
		cfg.User = u.User.Username()
		if password, ok := u.User.Password(); ok {
			cfg.Password = password
		}
	}
	if u.Path != "" {
		cfg.Database = strings.TrimPrefix(u.Path, "/")
	}

	return cfg
}
