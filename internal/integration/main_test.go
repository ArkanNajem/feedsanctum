package integration

import (
	"os"
	"testing"

	"github.com/ArkanNajem/feedsanctum/internal/handlers"
	"github.com/ArkanNajem/feedsanctum/internal/testingutils"
	_ "github.com/lib/pq"
)

var testCfg *handlers.APIConfig

func TestMain(m *testing.M) {
	queries, db, cleanup := testingutils.SetupTestDB()
	testCfg = &handlers.APIConfig{
		DB:     queries,
		DBConn: db,
	}

	code := m.Run()

	cleanup()

	os.Exit(code)
}
