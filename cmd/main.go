// @title           myCart API
// @version         1.0
// @description     Open source shopping-cart backend API - a single-binary e-commerce solution
// @termsOfService  https://github.com/shurco/mycart

// @contact.name   API Support
// @contact.url    https://github.com/shurco/mycart/issues
// @contact.email  support@mycart.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	app "github.com/shurco/mycart/internal"
	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/dbtransfer"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/update"

	_ "github.com/shurco/mycart/docs/swagger"
)

var (
	version   = "v0.0.1"
	gitCommit = "00000000"
	buildDate = "14.07.2023"
)

// dbOverrides carries the persistent --db-* flags. Values left empty fall
// through to the environment, lc_base/config.json and finally the built-in
// SQLite default.
var dbOverrides database.Overrides

// resolveDB resolves the database configuration for a command. The resolution
// order is: flags, environment, lc_base/config.json, SQLite default.
func resolveDB() database.Config {
	cfg, err := database.Resolve(dbOverrides)
	if err != nil {
		handleCommandError(err)
	}
	return cfg
}

var rootCmd = &cobra.Command{
	Use:                "mycart",
	Short:              "myCart CLI",
	Long:               "🛒 myCart - shopping-cart in 1 file",
	Version:            fmt.Sprintf("myCart %s (%s) from %s", version, gitCommit, buildDate),
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
}

func main() {
	update.SetVersion(&update.Version{
		CurrentVersion: version,
		GitCommit:      gitCommit,
		BuildDate:      buildDate,
	})

	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	rootCmd.PersistentFlags().StringVar(&dbOverrides.Driver, "db", "",
		`database: "sqlite" (default) or "postgres"`)
	rootCmd.PersistentFlags().StringVar(&dbOverrides.DSN, "db-dsn", "",
		`database connection string: a PostgreSQL DSN, or a SQLite file path`)

	rootCmd.AddCommand(cmdInit())
	rootCmd.AddCommand(cmdInstall())
	rootCmd.AddCommand(cmdServe())
	rootCmd.AddCommand(cmdUpdate())
	rootCmd.AddCommand(cmdMigrate())
	rootCmd.AddCommand(cmdDB())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// handleCommandError handles command execution errors uniformly.
func handleCommandError(err error) {
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}

// cmdServe creates and returns the serve command.
func cmdServe() *cobra.Command {
	var noSite, devMode bool
	var httpAddr, httpsAddr string

	cmd := &cobra.Command{
		Use:   "serve [flags]",
		Short: "Starts the web server (default to 0.0.0.0:8080)",
		Run: func(_ *cobra.Command, _ []string) {
			handleCommandError(app.NewApp(resolveDB(), httpAddr, httpsAddr, noSite, devMode))
		},
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http", "0.0.0.0:8080", "server address")
	cmd.PersistentFlags().StringVar(&httpsAddr, "https", "", "https server address (auto TLS)")
	cmd.PersistentFlags().BoolVar(&noSite, "no-site", false, "disable create site")
	cmd.PersistentFlags().BoolVar(&devMode, "dev", false, "develop mode")

	if err := cmd.PersistentFlags().MarkHidden("dev"); err != nil {
		fmt.Println("warning: failed to hide dev flag:", err)
	}

	return cmd
}

// cmdInstall creates and returns the install command.
func cmdInstall() *cobra.Command {
	var email, password, domain string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Create the admin account (first-time setup)",
		Run: func(_ *cobra.Command, _ []string) {
			handleCommandError(app.InstallAdmin(context.Background(), resolveDB(), &models.Install{
				Email:    email,
				Password: password,
				Domain:   domain,
			}))
			fmt.Println("Cart installed successfully")
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "admin email address")
	cmd.Flags().StringVar(&password, "password", "", "admin password (6-72 chars)")
	cmd.Flags().StringVar(&domain, "domain", "localhost", "public store domain")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")

	return cmd
}

// cmdInit creates and returns the init command.
func cmdInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Creating the basic structure",
		Run: func(_ *cobra.Command, _ []string) {
			handleCommandError(app.Init(resolveDB()))
		},
	}
}

// cmdUpdate creates and returns the update command.
func cmdUpdate() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Updating the application to the latest version",
		Run: func(_ *cobra.Command, _ []string) {
			cfg := &update.Config{
				Owner:             "shurco",
				Repo:              "mycart",
				CurrentVersion:    version,
				ArchiveExecutable: "mycart",
			}

			if err := update.Init(cfg); err != nil {
				handleCommandError(err)
				return
			}

			handleCommandError(app.Migrate(resolveDB()))
		},
	}
}

// cmdDB creates the parent of the database backup, restore and copy commands.
//
// They exist next to the configured database rather than inside it: `backup`
// and `restore` act on the database this installation uses (so `--db` and
// `--db-dsn` apply to them like everywhere else), while `copy` names its two
// ends explicitly, because the whole point of a copy is that they differ.
func cmdDB() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Back up, restore and copy the database",
	}

	cmd.AddCommand(cmdDBBackup())
	cmd.AddCommand(cmdDBRestore())
	cmd.AddCommand(cmdDBCopy())

	return cmd
}

// cmdDBBackup creates and returns the backup command.
func cmdDBBackup() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:   "backup [flags]",
		Short: "Write a dump of the PostgreSQL database to a file",
		Long: "Write the contents of the configured PostgreSQL database to a file.\n\n" +
			"The result is a SQL script: `psql -f FILE` replays it into a migrated, empty\n" +
			"database, and `mycart db restore --from FILE` reads it back without psql.\n" +
			"A path ending in .gz is compressed.",
		Run: func(_ *cobra.Command, _ []string) {
			path := out
			if path == "" {
				path = fmt.Sprintf("mycart-%s.sql.gz", time.Now().Format("20060102-150405"))
			}

			manifest, err := app.BackupDatabase(context.Background(), resolveDB(), path)
			if err != nil {
				handleCommandError(err)
				return
			}
			printDumpReport("Backed up", manifest, "to "+path)
		},
	}

	cmd.Flags().StringVar(&out, "out", "", "file to write (default mycart-<timestamp>.sql.gz)")

	return cmd
}

// cmdDBRestore creates and returns the restore command.
func cmdDBRestore() *cobra.Command {
	var from string
	var force bool

	cmd := &cobra.Command{
		Use:   "restore [flags]",
		Short: "Replace the PostgreSQL database with a dump",
		Long: "Migrate the configured PostgreSQL database and replace its contents with a\n" +
			"dump written by `mycart db backup`.\n\n" +
			"The whole load runs in one transaction: a dump that is truncated or does not\n" +
			"fit the schema leaves the database as it was. A database that already holds\n" +
			"an installation is refused unless --force is given, because restoring into it\n" +
			"erases the shop that is in there.",
		Run: func(_ *cobra.Command, _ []string) {
			manifest, err := app.RestoreDatabase(context.Background(), resolveDB(), from, force)
			if err != nil {
				handleCommandError(err)
				return
			}
			printDumpReport("Restored", manifest, "from "+from)
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "file to read (a .gz file is decompressed)")
	cmd.Flags().BoolVar(&force, "force", false, "restore over an installation, erasing it")
	_ = cmd.MarkFlagRequired("from")

	return cmd
}

// cmdDBCopy creates and returns the copy command.
func cmdDBCopy() *cobra.Command {
	var from, to string
	var dryRun, force bool

	cmd := &cobra.Command{
		Use:   "copy [flags]",
		Short: "Copy a cart into a PostgreSQL database",
		Long: "Copy every row of --from into --to, which is the configured database when\n" +
			"--to is not given.\n\n" +
			"--from is a SQLite file (spelled sqlite:PATH to be explicit) or a PostgreSQL\n" +
			"connection string, --to is a PostgreSQL connection string. This is how an existing\n" +
			"SQLite cart is moved onto PostgreSQL.\n\n" +
			"The target is migrated first and emptied table by table, and the rows are\n" +
			"verified with COUNT(*) as they land. Like `restore`, it refuses a target that\n" +
			"already holds an installation unless --force is given.",
		Run: func(_ *cobra.Command, _ []string) {
			src, err := dbConfigFrom(from)
			if err != nil {
				handleCommandError(err)
				return
			}

			dst := resolveDB()
			if strings.TrimSpace(to) != "" {
				if dst, err = dbConfigFrom(to); err != nil {
					handleCommandError(err)
					return
				}
			}

			// The target is checked before it is touched: migrating a SQLite
			// destination would create the file a copy then refuses.
			if err := dbtransfer.CheckCopyTarget(dst); err != nil {
				handleCommandError(fmt.Errorf("copy into %s: %w", dst.Redacted(), err))
				return
			}

			// A copy into a database that was never migrated has nothing to
			// write into, so the target is brought up to date first — the same
			// thing `restore` does.
			if !dryRun {
				if err := app.Migrate(dst); err != nil {
					handleCommandError(fmt.Errorf("migrate the target: %w", err))
					return
				}
			}

			manifest, err := app.CopyDatabase(context.Background(), src, dst, dbtransfer.CopyOptions{
				DryRun:  dryRun,
				Replace: force,
			})
			if err != nil {
				handleCommandError(err)
				return
			}

			verb := "Copied"
			if dryRun {
				verb = "Would copy"
			}
			printDumpReport(verb, manifest, "into "+dst.Redacted())
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "SQLite file (or sqlite:PATH) or PostgreSQL connection string to read")
	cmd.Flags().StringVar(&to, "to", "", "PostgreSQL connection string to write (default: the configured database)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be copied, without writing")
	cmd.Flags().BoolVar(&force, "force", false, "copy over an installation, erasing it")
	_ = cmd.MarkFlagRequired("from")

	return cmd
}

// dbConfigFrom reads a connection argument: a PostgreSQL DSN, or anything else
// as the path of a SQLite database. The `sqlite:` prefix is accepted so a path
// can be spelled out, and so a script does not depend on which of the two a
// bare word happens to look like.
func dbConfigFrom(spec string) (database.Config, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return database.Config{}, fmt.Errorf("no database given")
	}

	switch {
	case strings.HasPrefix(spec, "postgres://"), strings.HasPrefix(spec, "postgresql://"):
		return database.Config{Driver: database.DriverPostgres, DSN: spec, Source: database.SourceFlag}, nil
	case strings.HasPrefix(spec, "sqlite:"):
		path := strings.TrimPrefix(spec, "sqlite:")
		if strings.TrimSpace(path) == "" {
			return database.Config{}, fmt.Errorf("%q names no SQLite file", spec)
		}
		return database.Config{Driver: database.DriverSQLite, DSN: path, Source: database.SourceFlag}, nil
	default:
		return database.Config{Driver: database.DriverSQLite, DSN: spec, Source: database.SourceFlag}, nil
	}
}

// printDumpReport shows what a transfer moved, table by table. The counts are
// the same ones the restore verified, so they double as the record of what
// happened.
func printDumpReport(verb string, manifest *dbtransfer.Manifest, where string) {
	fmt.Printf("%s %d rows in %d tables %s\n", verb, manifest.Rows, len(manifest.Tables), where)
	for _, t := range manifest.Tables {
		fmt.Printf("  %-28s %8d\n", t.Name, t.Rows)
	}
	if len(manifest.Absent) > 0 {
		fmt.Printf("  left empty, not in the dump: %s\n", strings.Join(manifest.Absent, ", "))
	}
}

// cmdMigrate creates and returns the migrate command.
func cmdMigrate() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Migrate on the latest version of database schema",
		Run: func(_ *cobra.Command, _ []string) {
			handleCommandError(app.Migrate(resolveDB()))
		},
	}
}
