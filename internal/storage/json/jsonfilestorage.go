package json

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"stfg/internal/storage"
)

const (
	groceriesFile  = "groceries.json"
	flyersIndex    = "flyers_index.json"
	flyerItemsBase = "flyer_" // flyer_<id>.json
)

type FileStorage struct {
	dir      string
	muByPath sync.Map // map[string]*sync.Mutex
}

// NewJSON returns the JSON-backed storage. The cache directory is resolved
// from storage.CacheDir() and created with mode 0o700. Files are written
// with mode 0o600.
func NewJSON(ctx context.Context) (*FileStorage, error) {
	dir, err := storage.CacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f := &FileStorage{dir: dir}
	if err := f.Migrate(ctx); err != nil {
		return nil, err
	}
	return f, nil
}

// NewJSONAt is a test-only constructor that uses a specific directory.
// The public API stays NewJSON(ctx). This is the only test-only helper.
func NewJSONAt(ctx context.Context, dir string) (*FileStorage, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f := &FileStorage{dir: dir}
	if err := f.Migrate(ctx); err != nil {
		return nil, err
	}
	return f, nil
}

// Compile-time assertion that *FileStorage satisfies storage.Storage.
// If *FileStorage ever stops implementing the interface (e.g. a method
// signature changed), this line will produce a clear compile error.
var _ storage.Storage = (*FileStorage)(nil)

// --- Lifecycle ---

func (f *FileStorage) Close() error { return nil }

func (f *FileStorage) Migrate(ctx context.Context) error {
	if err := f.ensureEmpty(f.groceriesPath(), []storage.GroceryItem{}); err != nil {
		return err
	}
	if err := f.migrateFlyersIndexLegacy(); err != nil {
		return err
	}
	if err := f.ensureEmpty(f.flyersIndexPath(), []storage.Flyer{}); err != nil {
		return err
	}
	return nil
}

// --- Legacy schema migration ---

// legacyFlyer mirrors the pre-storage-interface RetailGroup on-disk schema,
// which used camelCase JSON keys (validFrom/validTo/location) and RFC3339
// string dates instead of time.Time. migrateFlyersIndexLegacy translates such
// files into the canonical storage.Flyer form and rewrites them in place.
//
// Unknown fields (such as the deprecated "aux" map) are ignored rather than
// destroyed: migration is best-effort and returns an error without rewriting
// the file if it cannot parse, so no data is ever lost.
//
// Detection is a substring scan for the legacy keys. It is idempotent because
// the canonical form only ever contains snake_case keys, so a second pass finds
// neither "validFrom" nor "location" and is a no-op.
//
// migrateFlyersIndexLegacy is per-path locked so concurrent constructors cannot
// race on the same index file.
type legacyFlyer struct {
	ID        int64         `json:"id"`
	ValidFrom string        `json:"validFrom"`
	ValidTo   string        `json:"validTo"`
	Name      string        `json:"name"`
	Merchant  string        `json:"merchant"`
	Locations []legacyStore `json:"location"`
}

type legacyStore struct {
	ID         int    `json:"id"`
	Address    string `json:"address"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postalCode"`
}

// parseLegacyTime parses a date from a legacy flyers_index entry. The on-disk
// value was historically either a full RFC3339 timestamp or a bare date, so we
// accept both rather than silently zeroing the field (or failing migration) on
// a format mismatch.
func parseLegacyTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

func (f *FileStorage) migrateFlyersIndexLegacy() error {
	path := f.flyersIndexPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No index file yet; ensureEmpty will seed the canonical form.
			return nil
		}
		return err
	}

	// Only migrate when the file actually carries the legacy camelCase schema.
	if !bytes.Contains(data, []byte(`"validFrom"`)) && !bytes.Contains(data, []byte(`"location"`)) {
		return nil
	}

	var legacy []legacyFlyer
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("migrate legacy flyers index: %w", err)
	}

	flyers := make([]storage.Flyer, 0, len(legacy))
	for _, lf := range legacy {
		from, err := parseLegacyTime(lf.ValidFrom)
		if err != nil {
			return fmt.Errorf("migrate legacy flyer %d: invalid validFrom %q: %w", lf.ID, lf.ValidFrom, err)
		}
		to, err := parseLegacyTime(lf.ValidTo)
		if err != nil {
			return fmt.Errorf("migrate legacy flyer %d: invalid validTo %q: %w", lf.ID, lf.ValidTo, err)
		}
		stores := make([]storage.Store, 0, len(lf.Locations))
		for _, ls := range lf.Locations {
			stores = append(stores, storage.Store{
				ID:         ls.ID,
				Address:    ls.Address,
				City:       ls.City,
				Province:   ls.Province,
				PostalCode: ls.PostalCode,
			})
		}
		flyers = append(flyers, storage.Flyer{
			ID:        lf.ID,
			ValidFrom: from,
			ValidTo:   to,
			Name:      lf.Name,
			Merchant:  lf.Merchant,
			Stores:    stores,
		})
	}
	// Rewrites the file atomically via the existing writeFile; on failure the
	// original file is left untouched (no data destruction).
	return f.writeFile(path, flyers)
}

// --- File path helpers (private) ---

func (f *FileStorage) groceriesPath() string   { return filepath.Join(f.dir, groceriesFile) }
func (f *FileStorage) flyersIndexPath() string { return filepath.Join(f.dir, flyersIndex) }
func (f *FileStorage) flyerItemsPath(flyerID int64) string {
	return filepath.Join(f.dir, fmt.Sprintf("%s%d.json", flyerItemsBase, flyerID))
}

func (f *FileStorage) ensureEmpty(path string, seed any) error {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return f.writeFile(path, seed)
	}
	return nil
}

// --- Lock + atomic write helpers ---

func (f *FileStorage) lockFor(path string) *sync.Mutex {
	actual, _ := f.muByPath.LoadOrStore(path, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func (f *FileStorage) writeFile(path string, data any) error {
	tmp := path + ".tmp"
	fh, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(fh)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fh.Close()
		os.Remove(tmp)
		return err
	}
	if err := fh.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func (f *FileStorage) readFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s", storage.ErrNotFound, path)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// lockPath acquires the per-path mutex for the given path.
// Callers must call unlockPath with the returned mutex.
func (f *FileStorage) lockPath(path string) *sync.Mutex {
	actual, _ := f.muByPath.LoadOrStore(path, &sync.Mutex{})
	mu := actual.(*sync.Mutex)
	mu.Lock()
	return mu
}

func (f *FileStorage) unlockPath(mu *sync.Mutex) {
	mu.Unlock()
}

// --- Grocery CRUD ---

func (f *FileStorage) loadGroceries(path string) ([]storage.GroceryItem, error) {
	var items []storage.GroceryItem
	if err := f.readFile(path, &items); err != nil {
		// Legacy files written by the old db.SaveGroceries stored a bare
		// []string of names. Fall back to that shape so pre-existing data
		// still loads instead of hard-failing on a schema change.
		if errors.Is(err, storage.ErrNotFound) {
			return []storage.GroceryItem{}, nil
		}
		var names []string
		if perr := f.readFile(path, &names); perr == nil {
			out := make([]storage.GroceryItem, 0, len(names))
			for _, n := range names {
				out = append(out, storage.GroceryItem{Name: n})
			}
			return out, nil
		}
		// Neither schema fit; surface the original error without destroying data.
		return nil, err
	}
	if items == nil {
		return []storage.GroceryItem{}, nil
	}
	return items, nil
}

func (f *FileStorage) AddGrocery(ctx context.Context, item storage.GroceryItem) error {
	path := f.groceriesPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	groceries, err := f.loadGroceries(path)
	if err != nil {
		return err
	}
	for _, existing := range groceries {
		if strings.EqualFold(existing.Name, item.Name) {
			return fmt.Errorf("%w: %s", storage.ErrDuplicate, item.Name)
		}
	}
	groceries = append(groceries, item)
	return f.writeFile(path, groceries)
}

func (f *FileStorage) RemoveGrocery(ctx context.Context, name string) error {
	path := f.groceriesPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	groceries, err := f.loadGroceries(path)
	if err != nil {
		return err
	}
	found := false
	filtered := make([]storage.GroceryItem, 0, len(groceries))
	for _, existing := range groceries {
		if strings.EqualFold(existing.Name, name) {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}
	if !found {
		return fmt.Errorf("%w: %s", storage.ErrNotFound, name)
	}
	return f.writeFile(path, filtered)
}

func (f *FileStorage) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) {
	path := f.groceriesPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)
	return f.loadGroceries(path)
}

func (f *FileStorage) HasGrocery(ctx context.Context, name string) (bool, error) {
	groceries, err := f.ListGroceries(ctx)
	if err != nil {
		return false, err
	}
	for _, existing := range groceries {
		if strings.EqualFold(existing.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

// --- Flyer CRUD ---

func (f *FileStorage) loadFlyers(path string) ([]storage.Flyer, error) {
	var flyers []storage.Flyer
	err := f.readFile(path, &flyers)
	if errors.Is(err, storage.ErrNotFound) {
		return []storage.Flyer{}, nil
	}
	if err != nil {
		return nil, err
	}
	if flyers == nil {
		return []storage.Flyer{}, nil
	}
	return flyers, nil
}

func (f *FileStorage) AddFlyer(ctx context.Context, flyer storage.Flyer) error {
	path := f.flyersIndexPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	flyers, err := f.loadFlyers(path)
	if err != nil {
		return err
	}
	for _, existing := range flyers {
		if existing.ID == flyer.ID {
			return fmt.Errorf("%w: %d", storage.ErrDuplicate, flyer.ID)
		}
	}
	flyers = append(flyers, flyer)
	return f.writeFile(path, flyers)
}

func (f *FileStorage) RemoveFlyer(ctx context.Context, id int64) error {
	path := f.flyersIndexPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	flyers, err := f.loadFlyers(path)
	if err != nil {
		return err
	}
	found := false
	filtered := make([]storage.Flyer, 0, len(flyers))
	for _, existing := range flyers {
		if existing.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}
	if !found {
		return fmt.Errorf("%w: %d", storage.ErrNotFound, id)
	}
	if err := f.writeFile(path, filtered); err != nil {
		return err
	}
	os.Remove(f.flyerItemsPath(id))
	return nil
}

func (f *FileStorage) GetFlyer(ctx context.Context, id int64) (*storage.Flyer, error) {
	flyers, err := f.ListFlyers(ctx)
	if err != nil {
		return nil, err
	}
	for _, flyer := range flyers {
		if flyer.ID == id {
			return &flyer, nil
		}
	}
	return nil, fmt.Errorf("%w: %d", storage.ErrNotFound, id)
}

func (f *FileStorage) ListFlyers(ctx context.Context) ([]storage.Flyer, error) {
	path := f.flyersIndexPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)
	return f.loadFlyers(path)
}

func (f *FileStorage) HasFlyer(ctx context.Context, id int64) (bool, error) {
	flyers, err := f.ListFlyers(ctx)
	if err != nil {
		return false, err
	}
	for _, flyer := range flyers {
		if flyer.ID == id {
			return true, nil
		}
	}
	return false, nil
}

// --- FlyerItem CRUD ---

func (f *FileStorage) loadFlyerItems(path string) ([]storage.FlyerItem, error) {
	var items []storage.FlyerItem
	err := f.readFile(path, &items)
	if errors.Is(err, storage.ErrNotFound) {
		return []storage.FlyerItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	if items == nil {
		return []storage.FlyerItem{}, nil
	}
	return items, nil
}

func (f *FileStorage) AddFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	path := f.flyerItemsPath(item.FlyerID)
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	items, err := f.loadFlyerItems(path)
	if err != nil {
		return err
	}
	for _, existing := range items {
		if existing.FlyerID == item.FlyerID && existing.ID == item.ID {
			return fmt.Errorf("%w: %d", storage.ErrDuplicate, item.ID)
		}
	}
	items = append(items, item)
	return f.writeFile(path, items)
}

func (f *FileStorage) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error {
	path := f.flyerItemsPath(flyerID)
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	items, err := f.loadFlyerItems(path)
	if err != nil {
		return err
	}
	found := false
	filtered := make([]storage.FlyerItem, 0, len(items))
	for _, existing := range items {
		if existing.ID == itemID {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}
	if !found {
		return fmt.Errorf("%w: %d", storage.ErrNotFound, itemID)
	}
	return f.writeFile(path, filtered)
}

func (f *FileStorage) ListFlyerItems(ctx context.Context, flyerID int64) ([]storage.FlyerItem, error) {
	path := f.flyerItemsPath(flyerID)
	mu := f.lockPath(path)
	defer f.unlockPath(mu)
	return f.loadFlyerItems(path)
}

func (f *FileStorage) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) {
	items, err := f.ListFlyerItems(ctx, flyerID)
	if err != nil {
		return false, err
	}
	for _, existing := range items {
		if existing.ID == itemID {
			return true, nil
		}
	}
	return false, nil
}

// --- Maintenance ---

func (f *FileStorage) PruneExpired(ctx context.Context, now time.Time) error {
	path := f.flyersIndexPath()
	mu := f.lockPath(path)
	defer f.unlockPath(mu)

	flyers, err := f.loadFlyers(path)
	if err != nil {
		return err
	}
	current := make([]storage.Flyer, 0, len(flyers))
	for _, flyer := range flyers {
		if !flyer.ValidTo.IsZero() && flyer.ValidTo.Before(now) {
			os.Remove(f.flyerItemsPath(flyer.ID))
			continue
		}
		current = append(current, flyer)
	}
	return f.writeFile(path, current)
}
