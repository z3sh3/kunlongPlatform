package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// moduleTables is the full set of console-module tables. Every dialect must
// create all of them, and running MigrateAll twice must stay idempotent.
var moduleTables = []string{
	"datasets",
	"dataset_grants",
	"finetune_jobs",
	"agents",
	"training_courses",
	"training_enrollments",
	"incubation_programs",
	"incubation_applications",
}

func TestConsoleModuleMigrationMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var dialector gorm.Dialector
			switch dialect {
			case "sqlite":
				dialector = sqlite.Open(filepath.Join(t.TempDir(), "modules.db"))
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("test database DSN is not configured")
				}
				dialector = gormmysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("test database DSN is not configured")
				}
				dialector = gormpostgres.Open(dsn)
			}

			db, err := gorm.Open(dialector, &gorm.Config{})
			require.NoError(t, err)

			previousDB := model.DB
			model.DB = db.Session(&gorm.Session{})
			t.Cleanup(func() { model.DB = previousDB })

			// Run twice to prove migration idempotency.
			for range 2 {
				require.NoError(t, MigrateAll())
			}

			for _, table := range moduleTables {
				assert.True(t, db.Migrator().HasTable(table),
					fmt.Sprintf("table %s should exist on %s", table, dialect))
			}

			// Existing data must survive a second migration pass.
			require.NoError(t, db.Exec(
				"INSERT INTO datasets (name, status, price_mode, unit_price, size_bytes) VALUES (?, ?, ?, ?, ?)",
				"survivor", 1, 0, 0, 0,
			).Error)
			require.NoError(t, MigrateAll())
			var count int64
			require.NoError(t, db.Raw("SELECT COUNT(*) FROM datasets WHERE name = ?", "survivor").Scan(&count).Error)
			assert.Equal(t, int64(1), count, "data must survive repeated migrations")
		})
	}
}
