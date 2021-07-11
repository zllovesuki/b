package backend

import (
	"context"
	"time"

	"github.com/zllovesuki/b/app"

	"github.com/pkg/errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// SQLiteData is the data model for storing bytes in SQLite
type SQLiteData struct {
	ID      string `gorm:"primaryKey"`
	Data    []byte
	Created time.Time
	Expires time.Time
}

// TableName is used for gorm.io only, overwriting the default table name
func (SQLiteData) TableName() string {
	return "backend_sqlite_data"
}

// SQLiteBackend implements app.Backend to persist KV data
type SQLiteBackend struct {
	db *gorm.DB
}

var _ app.Backend = &SQLiteBackend{}
var _ app.Removable = &SQLiteBackend{}

// NewSQLiteBackend returns a SQLite backend for the application
func NewSQLiteBackend(dbPath string) (*SQLiteBackend, error) {
	if dbPath == "" {
		return nil, errors.New("sqlite db path cannot be empty")
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		return nil, errors.Wrap(err, "opening sqlite db")
	}
	if err := db.AutoMigrate(&SQLiteData{}); err != nil {
		return nil, errors.Wrap(err, "auto migration")
	}
	return &SQLiteBackend{
		db: db,
	}, nil
}

func (s *SQLiteBackend) SaveTTL(c context.Context, identifier string, data []byte, ttl time.Duration) error {
	return s.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var d SQLiteData
		res := tx.First(&d, "id = ?", identifier)
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			// fall through
		} else if res.Error != nil {
			return res.Error
		}
		if !d.Created.IsZero() || (!d.Expires.IsZero() && time.Now().UTC().Before(d.Expires)) {
			return app.ErrConflict
		}
		d = SQLiteData{
			ID:      identifier,
			Data:    data,
			Created: time.Now().UTC(),
		}
		if ttl > 0 {
			d.Expires = time.Now().UTC().Add(ttl)
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"data", "expires"}),
		}).Create(&d).Error
	})
}

func (s *SQLiteBackend) Retrieve(c context.Context, identifier string) ([]byte, error) {
	var data []byte
	ret := s.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var d SQLiteData
		res := tx.First(&d, "id = ?", identifier)
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return app.ErrNotFound
		} else if res.Error != nil {
			return res.Error
		}
		if !d.Expires.IsZero() && time.Now().UTC().After(d.Expires) {
			return app.ErrNotFound
		}
		data = d.Data
		return nil
	})
	if ret != nil {
		return nil, errors.Wrap(ret, "unable to retrieve data")
	}
	return data, nil
}

func (s *SQLiteBackend) Close() error {
	return nil
}

func (s *SQLiteBackend) Delete(c context.Context, identifier string) error {
	return s.db.WithContext(c).Delete(&SQLiteData{}, "id = ?", identifier).Error
}
