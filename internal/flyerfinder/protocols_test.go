package flyerfinder

import (
	"stfg/internal/storage"
)

// MockFinder is a mock implementation of FlyerFinder for testing.
type MockFinder struct {
	FindFlyersFunc    func(postalCode string) ([]storage.Flyer, error)
	FindFlyerItemsFunc func(flyerID int64) ([]storage.FlyerItem, error)
}

func (m *MockFinder) FindFlyers(postalCode string) ([]storage.Flyer, error) {
	if m.FindFlyersFunc != nil {
		return m.FindFlyersFunc(postalCode)
	}
	return nil, nil
}

func (m *MockFinder) FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error) {
	if m.FindFlyerItemsFunc != nil {
		return m.FindFlyerItemsFunc(flyerID)
	}
	return nil, nil
}

// Test that MockFinder satisfies the FlyerFinder interface (compile-time check).
var _ FlyerFinder = (*MockFinder)(nil)