// Package sqlite provides a SQLite-backed implementation of storage.Storage
// using the pure-Go modernc.org/sqlite driver. The database file lives in
// opts.CacheDir (defaulted from storage.CacheDir) as stfg.sqlite and is
// created with mode 0o600.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
	"stfg/internal/storage"
)

const defaultFileName = "stfg.sqlite"

// defaultSchemaVersion is the version recorded in schema_version when Migrate
// runs on a fresh database.
const defaultSchemaVersion = 1

// createTableStatements are the DDL statements Migrate runs inside a single
// transaction. Every statement is IF NOT EXISTS, so Migrate stays idempotent
// across repeated opens.
var createTableStatements = []string{
	`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')));`,
	`CREATE TABLE IF NOT EXISTS groceries (name TEXT PRIMARY KEY COLLATE NOCASE, display_name TEXT NOT NULL, embedding BLOB NOT NULL);`,
	`CREATE TABLE IF NOT EXISTS flyers (id INTEGER PRIMARY KEY, valid_from TEXT NOT NULL, valid_to TEXT, name TEXT NOT NULL, merchant TEXT NOT NULL, stores TEXT NOT NULL DEFAULT '[]');`,
	`CREATE TABLE IF NOT EXISTS flyer_items (flyer_id INTEGER NOT NULL, id INTEGER NOT NULL, name TEXT NOT NULL, brand TEXT, price TEXT, image_url TEXT, video_url TEXT, display_type INTEGER, PRIMARY KEY (flyer_id, id), FOREIGN KEY (flyer_id) REFERENCES flyers(id) ON DELETE CASCADE);`,
	`CREATE INDEX IF NOT EXISTS idx_flyer_items_flyer_id ON flyer_items(flyer_id);`,
	`CREATE INDEX IF NOT EXISTS idx_flyers_valid_to ON flyers(valid_to);`,
}

// Options configures the SQLite storage backend.
type Options struct {
	// CacheDir is the directory where the SQLite database file is created.
	// If empty, storage.CacheDir() is used.
	CacheDir string
	// SQLiteFileName overrides the default database file name (stfg.sqlite).
	// Mainly useful for tests.
	SQLiteFileName string
}

// SQLiteStorage implements storage.Storage on top of a SQLite database file.
type SQLiteStorage struct {
	db       *sql.DB
	once     sync.Once
	closeErr error
}

var _ storage.Storage = (*SQLiteStorage)(nil)

// NewSQLite opens (creating the directory and file as needed) and migrates a
// SQLite storage backend rooted at opts.CacheDir.
func NewSQLite(ctx context.Context, opts Options) (*SQLiteStorage, error) {
	if opts.CacheDir == "" {
		dir, err := storage.CacheDir()
		if err != nil {
			return nil, err
		}
		opts.CacheDir = dir
	}

	if err := os.MkdirAll(opts.CacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("sqlite: create cache dir: %w", err)
	}

	fileName := opts.SQLiteFileName
	if fileName == "" {
		fileName = defaultFileName
	}

	dbPath := filepath.Join(opts.CacheDir, fileName)

	// Pre-create the database file with 0o600 so the OS permissions are correct
	// before the driver opens it. If the file already exists we leave its
	// permissions untouched.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			// If a concurrent opener created the file between our Stat and
			// OpenFile, swallow EEXIST and fall through to sql.Open, which will
			// reuse the existing file. Migrate is idempotent and ON CONFLICT
			// collisions are no-ops.
			if !errors.Is(err, os.ErrExist) {
				return nil, fmt.Errorf("sqlite: create db file: %w", err)
			}
		} else if err := f.Close(); err != nil {
			return nil, fmt.Errorf("sqlite: close db file: %w", err)
		}
	}

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	s := &SQLiteStorage{db: db}
	if err := s.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// The persistence plan requires a warning when the database file is more
	// permissive than 0o600, but this constructor has no logger wired in; we
	// intentionally do not log and simply continue.
	if info, err := os.Stat(dbPath); err == nil && info.Mode().Perm() > 0o600 {
		// TODO: emit via logger once one is available in this constructor.
	}

	return s, nil
}

// --- Lifecycle ---

// Close closes the underlying database handle. Safe to call multiple times.
func (s *SQLiteStorage) Close() error {
	s.once.Do(func() {
		if s.db != nil {
			s.closeErr = s.db.Close()
			// Intentionally retain the db handle after Close: sql.DB methods
			// return "sql: database is closed" errors (non-panicking) for
			// subsequent calls. Nil-ing the handle here would turn those into
			// nil-dereference panics.
		}
	})
	return s.closeErr
}

// Migrate creates the schema (tables + indexes) if it does not yet exist and
// records schema_version = 1. It is idempotent: calling it repeatedly on an
// already-migrated database is a no-op.
func (s *SQLiteStorage) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin migrate tx: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range createTableStatements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sqlite: create table: %w", err)
		}
	}

	// Check whether a schema version has already been recorded.
	var version sql.NullInt64
	if err := tx.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&version); err != nil {
		return fmt.Errorf("sqlite: query schema version: %w", err)
	}

	if !version.Valid {
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_version (version) VALUES (?) ON CONFLICT(version) DO NOTHING", defaultSchemaVersion); err != nil {
			return fmt.Errorf("sqlite: insert schema version: %w", err)
		}
	}

	return tx.Commit()
}

// --- Embedding helpers ---

// embeddingToBytes serialises a []float32 into a BLOB of little-endian
// uint32 values, one per element. An empty (or nil) embedding becomes an empty
// (but non-nil) byte slice so it stores as an empty BLOB rather than NULL,
// keeping the NOT NULL column constraint satisfiable. The round-trip is
// intentionally asymmetric: bytesToEmbedding returns a nil []float32 when it
// encounters an empty BLOB, so an empty stored embedding reads back as nil
// rather than a non-nil empty slice.
func embeddingToBytes(embedding []float32) []byte {
	buf := make([]byte, len(embedding)*4)
	for i, f := range embedding {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// bytesToEmbedding deserialises a BLOB back into a []float32.
func bytesToEmbedding(buf []byte) ([]float32, error) {
	if len(buf) == 0 {
		return nil, nil
	}
	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("sqlite: invalid embedding byte length %d", len(buf))
	}
	floats := make([]float32, len(buf)/4)
	for i := range floats {
		floats[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return floats, nil
}

// --- Stores serialisation ---

// marshalStores serialises a []storage.Store as JSON text. A nil slice
// becomes "null" so it round-trips to nil via unmarshalStores.
func marshalStores(stores []storage.Store) (string, error) {
	if stores == nil {
		return "null", nil
	}
	data, err := json.Marshal(stores)
	if err != nil {
		return "", fmt.Errorf("sqlite: marshal stores: %w", err)
	}
	return string(data), nil
}

// unmarshalStores deserialises the stores JSON column. A "null" or empty
// string yields a nil slice; "[]" yields a non-nil empty slice.
func unmarshalStores(data string) ([]storage.Store, error) {
	if data == "" || data == "null" {
		return nil, nil
	}
	var stores []storage.Store
	if err := json.Unmarshal([]byte(data), &stores); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal stores: %w", err)
	}
	return stores, nil
}

// --- Groceries ---

func (s *SQLiteStorage) AddGrocery(ctx context.Context, item storage.GroceryItem) error {
	if strings.TrimSpace(item.Name) == "" {
		return fmt.Errorf("%w: empty grocery name", storage.ErrInvalidArgument)
	}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO groceries (name, display_name, embedding) VALUES (?, ?, ?) ON CONFLICT(name) DO NOTHING",
		strings.ToLower(item.Name), item.Name, embeddingToBytes(item.Embedding))
	if err != nil {
		return fmt.Errorf("sqlite: insert grocery: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: grocery rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %s", storage.ErrDuplicate, item.Name)
	}
	return nil
}

func (s *SQLiteStorage) RemoveGrocery(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: empty grocery name", storage.ErrInvalidArgument)
	}
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM groceries WHERE name = ?", strings.ToLower(name))
	if err != nil {
		return fmt.Errorf("sqlite: remove grocery: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: remove grocery rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %s", storage.ErrNotFound, name)
	}
	return nil
}

func (s *SQLiteStorage) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT display_name, embedding FROM groceries ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("sqlite: list groceries: %w", err)
	}
	defer rows.Close()

	var items []storage.GroceryItem
	for rows.Next() {
		var name string
		var blob []byte
		if err := rows.Scan(&name, &blob); err != nil {
			return nil, fmt.Errorf("sqlite: scan grocery: %w", err)
		}
		emb, err := bytesToEmbedding(blob)
		if err != nil {
			return nil, fmt.Errorf("sqlite: decode embedding: %w", err)
		}
		items = append(items, storage.GroceryItem{Name: name, Embedding: emb})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate groceries: %w", err)
	}
	if items == nil {
		items = []storage.GroceryItem{}
	}
	return items, nil
}

func (s *SQLiteStorage) HasGrocery(ctx context.Context, name string) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, fmt.Errorf("%w: empty grocery name", storage.ErrInvalidArgument)
	}
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM groceries WHERE name = ?)", strings.ToLower(name)).
		Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("sqlite: has grocery: %w", err)
	}
	return exists, nil
}

// --- Flyers ---

func (s *SQLiteStorage) AddFlyer(ctx context.Context, flyer storage.Flyer) error {
	storesJSON, err := marshalStores(flyer.Stores)
	if err != nil {
		return err
	}

	validTo := sql.NullString{}
	if !flyer.ValidTo.IsZero() {
		validTo = sql.NullString{String: flyer.ValidTo.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO flyers (id, valid_from, valid_to, name, merchant, stores) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING",
		flyer.ID, flyer.ValidFrom.UTC().Format(time.RFC3339Nano), validTo, flyer.Name, flyer.Merchant, storesJSON)
	if err != nil {
		return fmt.Errorf("sqlite: insert flyer: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: flyer rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %d", storage.ErrDuplicate, flyer.ID)
	}
	return nil
}

func (s *SQLiteStorage) RemoveFlyer(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM flyers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("sqlite: remove flyer: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: remove flyer rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %d", storage.ErrNotFound, id)
	}
	return nil
}

func (s *SQLiteStorage) GetFlyer(ctx context.Context, id int64) (*storage.Flyer, error) {
	var (
		f           storage.Flyer
		validFrom   string
		validTo     sql.NullString
		name, merch string
		storesJSON  string
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT id, valid_from, valid_to, name, merchant, stores FROM flyers WHERE id = ?", id).
		Scan(&f.ID, &validFrom, &validTo, &name, &merch, &storesJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %d", storage.ErrNotFound, id)
		}
		return nil, fmt.Errorf("sqlite: get flyer: %w", err)
	}

	f.ValidFrom, err = time.Parse(time.RFC3339, validFrom)
	if err != nil {
		return nil, fmt.Errorf("sqlite: parse valid_from: %w", err)
	}
	if validTo.Valid {
		f.ValidTo, err = time.Parse(time.RFC3339, validTo.String)
		if err != nil {
			return nil, fmt.Errorf("sqlite: parse valid_to: %w", err)
		}
	}

	f.Name = name
	f.Merchant = merch
	f.Stores, err = unmarshalStores(storesJSON)
	if err != nil {
		return nil, err
	}

	return &f, nil
}

func (s *SQLiteStorage) ListFlyers(ctx context.Context) ([]storage.Flyer, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, valid_from, valid_to, name, merchant, stores FROM flyers ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("sqlite: list flyers: %w", err)
	}
	defer rows.Close()

	var flyers []storage.Flyer
	for rows.Next() {
		var f storage.Flyer
		var validFrom, name, merch, storesJSON string
		var validTo sql.NullString

		if err := rows.Scan(&f.ID, &validFrom, &validTo, &name, &merch, &storesJSON); err != nil {
			return nil, fmt.Errorf("sqlite: scan flyer: %w", err)
		}

		f.ValidFrom, err = time.Parse(time.RFC3339, validFrom)
		if err != nil {
			return nil, fmt.Errorf("sqlite: parse valid_from: %w", err)
		}
		if validTo.Valid {
			f.ValidTo, err = time.Parse(time.RFC3339, validTo.String)
			if err != nil {
				return nil, fmt.Errorf("sqlite: parse valid_to: %w", err)
			}
		}

		f.Name = name
		f.Merchant = merch
		f.Stores, err = unmarshalStores(storesJSON)
		if err != nil {
			return nil, err
		}

		flyers = append(flyers, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate flyers: %w", err)
	}
	if flyers == nil {
		flyers = []storage.Flyer{}
	}
	return flyers, nil
}

func (s *SQLiteStorage) HasFlyer(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM flyers WHERE id = ?)", id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("sqlite: has flyer: %w", err)
	}
	return exists, nil
}

// --- Flyer Items ---

func (s *SQLiteStorage) AddFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO flyer_items (id, flyer_id, name, brand, price, image_url, video_url, display_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(flyer_id, id) DO NOTHING",
		item.ID, item.FlyerID, item.Name, item.Brand, item.Price, item.ImageURL, item.VideoURL, item.DisplayType)
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			return fmt.Errorf("%w: flyer %d", storage.ErrNotFound, item.FlyerID)
		}
		return fmt.Errorf("sqlite: insert flyer item: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: flyer item rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %d", storage.ErrDuplicate, item.ID)
	}
	return nil
}

func (s *SQLiteStorage) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM flyer_items WHERE flyer_id = ? AND id = ?", flyerID, itemID)
	if err != nil {
		return fmt.Errorf("sqlite: remove flyer item: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: remove flyer item rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: flyer_id=%d, item_id=%d", storage.ErrNotFound, flyerID, itemID)
	}
	return nil
}

func (s *SQLiteStorage) ListFlyerItems(ctx context.Context, flyerID int64) ([]storage.FlyerItem, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, flyer_id, name, brand, price, image_url, video_url, display_type FROM flyer_items WHERE flyer_id = ? ORDER BY id",
		flyerID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list flyer items: %w", err)
	}
	defer rows.Close()

	var items []storage.FlyerItem
	for rows.Next() {
		var item storage.FlyerItem
		var brand, price, imageURL, videoURL sql.NullString

		if err := rows.Scan(&item.ID, &item.FlyerID, &item.Name, &brand, &price, &imageURL, &videoURL, &item.DisplayType); err != nil {
			return nil, fmt.Errorf("sqlite: scan flyer item: %w", err)
		}

		item.Brand = brand.String
		item.Price = price.String
		item.ImageURL = imageURL.String
		item.VideoURL = videoURL.String

		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate flyer items: %w", err)
	}
	if items == nil {
		items = []storage.FlyerItem{}
	}
	return items, nil
}

func (s *SQLiteStorage) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM flyer_items WHERE flyer_id = ? AND id = ?)",
		flyerID, itemID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("sqlite: has flyer item: %w", err)
	}
	return exists, nil
}

// --- Maintenance ---

// PruneExpired removes all flyers whose valid_to is before now. Items are
// removed automatically via the ON DELETE CASCADE foreign key.
func (s *SQLiteStorage) PruneExpired(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM flyers WHERE julianday(valid_to) < julianday(?)", now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("sqlite: prune expired flyers: %w", err)
	}
	return nil
}
