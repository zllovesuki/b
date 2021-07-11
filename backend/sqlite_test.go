package backend

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/apptest"
)

var p = filepath.Join(os.TempDir(), "b-sqlite-testing.db")

func getSQLiteFixtures(t *testing.T) (*SQLiteBackend, func()) {
	b, err := NewSQLiteBackend(p)
	require.NoError(t, err)

	rand.Seed(time.Now().Unix())

	return b, func() {
		db, err := b.db.DB()
		require.NoError(t, err)
		err = db.Close()
		require.NoError(t, err)
		os.Remove(p)
	}
}

func TestSQLiteBackend(t *testing.T) {
	b, cleanup := getSQLiteFixtures(t)
	defer cleanup()

	apptest.TestBackend(t, b)
}

func TestSQLiteDelete(t *testing.T) {
	b, cleanup := getSQLiteFixtures(t)
	defer cleanup()

	apptest.TestRemovableBackend(t, b)
}
