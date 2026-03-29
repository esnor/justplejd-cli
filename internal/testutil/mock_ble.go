// Package testutil provides mock implementations for testing.
package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/gloriaoracle/justplejd-cli/pkg/ble"
)

// WriteRecord records a write to a characteristic.
type WriteRecord struct {
	UUID string
	Data []byte
}

// MockDevice simulates a BLE GATT device.
type MockDevice struct {
	mu              sync.Mutex
	characteristics map[string][]byte
	Writes          []WriteRecord
	subscribers     map[string]func([]byte)
	writeHooks      map[string]func([]byte) []byte
	Disconnected    bool
}

// NewMockDevice creates a new mock device with initial characteristic values.
func NewMockDevice() *MockDevice {
	return &MockDevice{
		characteristics: make(map[string][]byte),
		subscribers:     make(map[string]func([]byte)),
		writeHooks:      make(map[string]func([]byte) []byte),
	}
}

// OnWrite registers a hook that transforms written data before storing.
// Used to simulate device behavior (e.g., ping increments the byte).
func (d *MockDevice) OnWrite(uuid string, hook func([]byte) []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writeHooks[uuid] = hook
}

// SetCharacteristic sets the value that will be returned when reading a characteristic.
func (d *MockDevice) SetCharacteristic(uuid string, data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.characteristics[uuid] = data
}

// WriteCharacteristic records the write and stores the value.
// If a write hook is registered for the UUID, the hook transforms the value before storing.
func (d *MockDevice) WriteCharacteristic(uuid string, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	copied := make([]byte, len(data))
	copy(copied, data)
	d.Writes = append(d.Writes, WriteRecord{UUID: uuid, Data: copied})
	stored := copied
	if hook, ok := d.writeHooks[uuid]; ok {
		stored = hook(copied)
	}
	d.characteristics[uuid] = stored
	return nil
}

// WriteCharacteristicWithResponse records the write and stores the value (same as WriteCharacteristic for mock).
func (d *MockDevice) WriteCharacteristicWithResponse(uuid string, data []byte) error {
	return d.WriteCharacteristic(uuid, data)
}

// ReadCharacteristic returns the stored value for the characteristic.
func (d *MockDevice) ReadCharacteristic(uuid string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, ok := d.characteristics[uuid]
	if !ok {
		return nil, fmt.Errorf("characteristic %s not found", uuid)
	}
	return data, nil
}

// Subscribe registers a notification callback for a characteristic.
func (d *MockDevice) Subscribe(uuid string, callback func([]byte)) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers[uuid] = callback
	return nil
}

// Disconnect marks the device as disconnected.
func (d *MockDevice) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Disconnected = true
	return nil
}

// InjectNotification simulates an incoming BLE notification.
func (d *MockDevice) InjectNotification(uuid string, data []byte) {
	d.mu.Lock()
	cb := d.subscribers[uuid]
	d.mu.Unlock()
	if cb != nil {
		cb(data)
	}
}

// GetWrites returns all writes to the given characteristic UUID.
func (d *MockDevice) GetWrites(uuid string) [][]byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	var result [][]byte
	for _, w := range d.Writes {
		if w.UUID == uuid {
			result = append(result, w.Data)
		}
	}
	return result
}

// MockAdapter simulates a BLE adapter for scanning and connecting.
type MockAdapter struct {
	ScanResults []ble.ScanResult
	Device      *MockDevice
	ScanErr     error
	ConnectErr  error
}

// NewMockAdapter creates a new mock BLE adapter.
func NewMockAdapter(device *MockDevice, scanResults ...ble.ScanResult) *MockAdapter {
	return &MockAdapter{
		ScanResults: scanResults,
		Device:      device,
	}
}

// Scan delivers stored scan results to the callback.
func (a *MockAdapter) Scan(ctx context.Context, callback func(ble.ScanResult)) error {
	if a.ScanErr != nil {
		return a.ScanErr
	}
	for _, r := range a.ScanResults {
		callback(r)
	}
	return nil
}

// StopScan is a no-op for the mock.
func (a *MockAdapter) StopScan() error {
	return nil
}

// Connect returns the mock device.
func (a *MockAdapter) Connect(address string) (ble.Device, error) {
	if a.ConnectErr != nil {
		return nil, a.ConnectErr
	}
	return a.Device, nil
}
