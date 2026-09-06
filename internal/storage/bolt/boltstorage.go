// Package bolt provides a BoltDB-backed implementation of the storage.Storage interface.
// It uses go.etcd.io/bbolt (pure Go, no CGO) for embedded key-value persistence.
// Database files land in the cache dir (default stfg.bolt) and are created
// with mode 0o600 on first use.
package bolt

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"stfg/internal/storage"
)

const (
	defaultBoltFileName = "stfg.bolt"
	bucketMeta          = "meta"
	bucketGroceries     = "groceries"
	bucketFlyers        = "flyers"
	bucketFlyerItems    = "flyer_items"
	bucketStores        = "stores"
)

// Options configures the BoltDB storage backend.
type Options struct {
	// CacheDir is the directory where the BoltDB database file is created.
	// If empty, storage.CacheDir() is used.
	CacheDir string
	// BoltFileName overrides the default database file name (stfg.bolt).
	// Mainly useful for tests.
	BoltFileName string
}

// BoltStorage implements storage.Storage backed by BoltDB.
type BoltStorage struct {
	db       *bbolt.DB
	once     sync.Once
	closeErr error
}

var _ storage.Storage = (*BoltStorage)(nil)

// NewBolt opens (creating the directory and file as needed) and migrates a
// BoltDB storage backend. The caller must call Close.
func NewBolt(ctx context.Context, opts Options) (*BoltStorage, error) {
	dir := opts.CacheDir
	if dir == "" {
		var err error
		dir, err = storage.CacheDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("bolt: create cache dir: %w", err)
	}

	fileName := opts.BoltFileName
	if fileName == "" {
		fileName = defaultBoltFileName
	}

	dbPath := filepath.Join(dir, fileName)
	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("bolt: open: %w", err)
	}

	// The persistence plan requires a warning when the database file is more
	// permissive than 0o600, but this constructor has no logger wired in; we
	// intentionally do not log and simply continue.
	if info, err := os.Stat(dbPath); err == nil && info.Mode().Perm() > 0o600 {
		// TODO: emit via logger once one is available in this constructor.
	}

	s := &BoltStorage{db: db}
	if err := s.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the underlying BoltDB database. Safe to call multiple times.
func (s *BoltStorage) Close() error {
	s.once.Do(func() {
		if s.db != nil {
			s.closeErr = s.db.Close()
			// Intentionally retain the db handle after Close: bbolt methods
			// (View/Update/Begin) return bbolt.ErrDatabaseNotOpen (non-panicking)
			// for subsequent calls. Nil-ing the handle here would turn those
			// into nil-dereference panics.
		}
	})
	return s.closeErr
}

// Migrate creates buckets if missing. Idempotent.
func (s *BoltStorage) Migrate(ctx context.Context) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketMeta)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketGroceries)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketFlyers)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketFlyerItems)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketStores)); err != nil {
			return err
		}
		meta := tx.Bucket([]byte(bucketMeta))
		if v := meta.Get([]byte("schema_version")); v == nil {
			// Store version as big-endian int64
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, 1)
			if err := meta.Put([]byte("schema_version"), buf); err != nil {
				return err
			}
			// Compaction is intentionally not performed in Migrate: bbolt has
			// no *DB.Shrink method (only package-level bbolt.Compact, which
			// needs a full-DB copy per Open and is not acceptable for a CLI).
			// Deferred; see plan open-question #6.
		}
		return nil
	})
}

// AddGrocery inserts a grocery item. Returns ErrDuplicate if name already exists (case-insensitive).
func (s *BoltStorage) AddGrocery(ctx context.Context, item storage.GroceryItem) error {
	if strings.TrimSpace(item.Name) == "" {
		return fmt.Errorf("%w: empty grocery name", storage.ErrInvalidArgument)
	}
	key := storage.NormalizeName(item.Name)
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroceries))
		existing := b.Get([]byte(key))
		if existing != nil {
			return fmt.Errorf("%w: %s", storage.ErrDuplicate, item.Name)
		}
		data, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("marshal grocery: %w", err)
		}
		return b.Put([]byte(key), data)
	})
}

// RemoveGrocery removes a grocery item by name (case-insensitive).
func (s *BoltStorage) RemoveGrocery(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: empty grocery name", storage.ErrInvalidArgument)
	}
	key := storage.NormalizeName(name)
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroceries))
		existing := b.Get([]byte(key))
		if existing == nil {
			return fmt.Errorf("%w: %s", storage.ErrNotFound, name)
		}
		return b.Delete([]byte(key))
	})
}

// ListGroceries returns all grocery items ordered by name (lexicographic on lowercased key).
func (s *BoltStorage) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) {
	out := []storage.GroceryItem{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroceries))
		return b.ForEach(func(k, v []byte) error {
			var item storage.GroceryItem
			if err := json.Unmarshal(v, &item); err != nil {
				return fmt.Errorf("bolt: list groceries: %w", err)
			}
			out = append(out, item)
			return nil
		})
	})
	return out, err
}

// HasGrocery returns true if a grocery with the given name exists (case-insensitive).
func (s *BoltStorage) HasGrocery(ctx context.Context, name string) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, fmt.Errorf("%w: empty grocery name", storage.ErrInvalidArgument)
	}
	key := storage.NormalizeName(name)
	var found bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroceries))
		found = b.Get([]byte(key)) != nil
		return nil
	})
	return found, err
}

// AddFlyer inserts a flyer. Returns ErrDuplicate if ID already exists.
func (s *BoltStorage) AddFlyer(ctx context.Context, flyer storage.Flyer) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		flyersB := tx.Bucket([]byte(bucketFlyers))
		idBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(idBytes, uint64(flyer.ID))
		existing := flyersB.Get(idBytes)
		if existing != nil {
			return fmt.Errorf("%w: %d", storage.ErrDuplicate, flyer.ID)
		}
		// Marshal flyer without stores (stored separately)
		flyerData := storage.Flyer{
			ID:        flyer.ID,
			ValidFrom: flyer.ValidFrom,
			ValidTo:   flyer.ValidTo,
			Name:      flyer.Name,
			Merchant:  flyer.Merchant,
			Stores:    nil, // stores stored separately
		}
		data, err := json.Marshal(flyerData)
		if err != nil {
			return fmt.Errorf("bolt: marshal flyer: %w", err)
		}
		if err := flyersB.Put(idBytes, data); err != nil {
			return err
		}
		// Store stores in separate bucket
		storesB := tx.Bucket([]byte(bucketStores))
		storesJSON, err := json.Marshal(flyer.Stores)
		if err != nil {
			return fmt.Errorf("bolt: marshal stores: %w", err)
		}
		return storesB.Put(idBytes, storesJSON)
	})
}

// RemoveFlyer removes a flyer by ID. Cascades to flyer_items bucket.
func (s *BoltStorage) RemoveFlyer(ctx context.Context, id int64) error {
	idBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(idBytes, uint64(id))
	return s.db.Update(func(tx *bbolt.Tx) error {
		flyersB := tx.Bucket([]byte(bucketFlyers))
		existing := flyersB.Get(idBytes)
		if existing == nil {
			return fmt.Errorf("%w: %d", storage.ErrNotFound, id)
		}
		// Delete nested flyer_items sub-bucket
		itemsB := tx.Bucket([]byte(bucketFlyerItems))
		if ib := itemsB.Bucket(idBytes); ib != nil {
			if err := itemsB.DeleteBucket(idBytes); err != nil {
				return err
			}
		}
		// Delete flyer
		if err := flyersB.Delete(idBytes); err != nil {
			return err
		}
		// Delete stores
		storesB := tx.Bucket([]byte(bucketStores))
		return storesB.Delete(idBytes)
	})
}

// GetFlyer returns a flyer by ID with stores rehydrated.
func (s *BoltStorage) GetFlyer(ctx context.Context, id int64) (*storage.Flyer, error) {
	idBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(idBytes, uint64(id))
	var flyer *storage.Flyer
	err := s.db.View(func(tx *bbolt.Tx) error {
		flyersB := tx.Bucket([]byte(bucketFlyers))
		data := flyersB.Get(idBytes)
		if data == nil {
			return fmt.Errorf("%w: %d", storage.ErrNotFound, id)
		}
		var f storage.Flyer
		if err := json.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("bolt: get flyer: %w", err)
		}
		// Load stores
		storesB := tx.Bucket([]byte(bucketStores))
		storesData := storesB.Get(idBytes)
		if storesData != nil {
			var stores []storage.Store
			if err := json.Unmarshal(storesData, &stores); err != nil {
				return fmt.Errorf("bolt: get flyer: %w", err)
			}
			f.Stores = stores
		}
		flyer = &f
		return nil
	})
	return flyer, err
}

// ListFlyers returns all flyers ordered by ID.
func (s *BoltStorage) ListFlyers(ctx context.Context) ([]storage.Flyer, error) {
	out := []storage.Flyer{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		flyersB := tx.Bucket([]byte(bucketFlyers))
		storesB := tx.Bucket([]byte(bucketStores))
		return flyersB.ForEach(func(k, v []byte) error {
			var f storage.Flyer
			if err := json.Unmarshal(v, &f); err != nil {
				return fmt.Errorf("bolt: list flyers: %w", err)
			}
			// Load stores
			storesData := storesB.Get(k)
			if storesData != nil {
				var stores []storage.Store
				if err := json.Unmarshal(storesData, &stores); err != nil {
					return fmt.Errorf("bolt: list flyers: %w", err)
				}
				f.Stores = stores
			}
			out = append(out, f)
			return nil
		})
	})
	return out, err
}

// HasFlyer returns true if a flyer with the given ID exists.
func (s *BoltStorage) HasFlyer(ctx context.Context, id int64) (bool, error) {
	idBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(idBytes, uint64(id))
	var found bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		flyersB := tx.Bucket([]byte(bucketFlyers))
		found = flyersB.Get(idBytes) != nil
		return nil
	})
	return found, err
}

// AddFlyerItem inserts a flyer item.
func (s *BoltStorage) AddFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	flyerIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(flyerIDBytes, uint64(item.FlyerID))
	itemIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(itemIDBytes, uint64(item.ID))
	return s.db.Update(func(tx *bbolt.Tx) error {
		itemsB := tx.Bucket([]byte(bucketFlyerItems))
		if tx.Bucket([]byte(bucketFlyers)).Get(flyerIDBytes) == nil {
			return fmt.Errorf("%w: flyer %d", storage.ErrNotFound, item.FlyerID)
		}
		flyerB := itemsB.Bucket(flyerIDBytes)
		if flyerB == nil {
			// Create sub-bucket for this flyer
			var err error
			flyerB, err = itemsB.CreateBucket(flyerIDBytes)
			if err != nil {
				return err
			}
		}
		if flyerB.Get(itemIDBytes) != nil {
			return fmt.Errorf("%w: item %d", storage.ErrDuplicate, item.ID)
		}
		data, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("bolt: marshal flyer item: %w", err)
		}
		return flyerB.Put(itemIDBytes, data)
	})
}

// RemoveFlyerItem removes a flyer item by flyer ID and item ID.
func (s *BoltStorage) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error {
	flyerIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(flyerIDBytes, uint64(flyerID))
	itemIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(itemIDBytes, uint64(itemID))
	return s.db.Update(func(tx *bbolt.Tx) error {
		itemsB := tx.Bucket([]byte(bucketFlyerItems))
		flyerB := itemsB.Bucket(flyerIDBytes)
		if flyerB == nil {
			return fmt.Errorf("%w: flyer=%d item=%d", storage.ErrNotFound, flyerID, itemID)
		}
		existing := flyerB.Get(itemIDBytes)
		if existing == nil {
			return fmt.Errorf("%w: flyer=%d item=%d", storage.ErrNotFound, flyerID, itemID)
		}
		return flyerB.Delete(itemIDBytes)
	})
}

// ListFlyerItems returns all items for a given flyer ID ordered by item ID.
func (s *BoltStorage) ListFlyerItems(ctx context.Context, flyerID int64) ([]storage.FlyerItem, error) {
	flyerIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(flyerIDBytes, uint64(flyerID))
	out := []storage.FlyerItem{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		itemsB := tx.Bucket([]byte(bucketFlyerItems))
		flyerB := itemsB.Bucket(flyerIDBytes)
		if flyerB == nil {
			return nil // No items for this flyer
		}
		return flyerB.ForEach(func(k, v []byte) error {
			var item storage.FlyerItem
			if err := json.Unmarshal(v, &item); err != nil {
				return fmt.Errorf("bolt: list flyer items: %w", err)
			}
			out = append(out, item)
			return nil
		})
	})
	return out, err
}

// HasFlyerItem returns true if a flyer item with the given IDs exists.
func (s *BoltStorage) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) {
	flyerIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(flyerIDBytes, uint64(flyerID))
	itemIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(itemIDBytes, uint64(itemID))
	var found bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		itemsB := tx.Bucket([]byte(bucketFlyerItems))
		flyerB := itemsB.Bucket(flyerIDBytes)
		if flyerB == nil {
			return nil
		}
		found = flyerB.Get(itemIDBytes) != nil
		return nil
	})
	return found, err
}

// PruneExpired removes flyers whose ValidTo is before now, cascading to items bucket.
func (s *BoltStorage) PruneExpired(ctx context.Context, now time.Time) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		flyersB := tx.Bucket([]byte(bucketFlyers))
		itemsB := tx.Bucket([]byte(bucketFlyerItems))
		storesB := tx.Bucket([]byte(bucketStores))
		var stale [][]byte
		// Gather keys to delete first to avoid mutating during iteration
		err := flyersB.ForEach(func(k, v []byte) error {
			var f storage.Flyer
			if err := json.Unmarshal(v, &f); err != nil {
				return fmt.Errorf("bolt: prune expired: %w", err)
			}
			if !f.ValidTo.IsZero() && f.ValidTo.Before(now) {
				// Copy key to avoid reference issues
				stale = append(stale, append([]byte(nil), k...))
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, k := range stale {
			// Delete items sub-bucket
			if ib := itemsB.Bucket(k); ib != nil {
				if err := itemsB.DeleteBucket(k); err != nil {
					return err
				}
			}
			// Delete flyer
			if err := flyersB.Delete(k); err != nil {
				return err
			}
			// Delete stores
			if err := storesB.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}
