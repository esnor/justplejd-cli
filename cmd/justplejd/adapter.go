package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gloriaoracle/justplejd-cli/pkg/ble"
	"tinygo.org/x/bluetooth"
)

// realAdapter wraps tinygo.org/x/bluetooth for actual BLE hardware.
type realAdapter struct {
	adapter *bluetooth.Adapter
	done    chan struct{}
}

func newBLEAdapter() (ble.Adapter, error) {
	adapter := bluetooth.DefaultAdapter
	// Retry Enable with backoff — CoreBluetooth may still be recovering
	// from a previous session's disconnect.
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		err = adapter.Enable()
		if err == nil {
			return &realAdapter{adapter: adapter, done: make(chan struct{})}, nil
		}
	}
	return nil, fmt.Errorf("enable bluetooth: %w (tried 3 times — is Bluetooth on?)", err)
}

func (a *realAdapter) Scan(ctx context.Context, callback func(ble.ScanResult)) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- a.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			mfrData := make(map[uint16][]byte)
			if result.HasServiceUUID(bluetooth.ServiceUUIDGenericAccess) || true {
				mds := result.ManufacturerData()
				for _, md := range mds {
					mfrData[md.CompanyID] = md.Data
				}
			}

			callback(ble.ScanResult{
				Address:          result.Address.String(),
				Name:             result.LocalName(),
				RSSI:             int(result.RSSI),
				ManufacturerData: mfrData,
			})
		})
	}()

	select {
	case <-ctx.Done():
		a.adapter.StopScan()
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *realAdapter) StopScan() error {
	return a.adapter.StopScan()
}

type realDevice struct {
	device  bluetooth.Device
	chars   map[string]bluetooth.DeviceCharacteristic
	adapter *bluetooth.Adapter
}

func (a *realAdapter) Connect(address string) (ble.Device, error) {
	addr := bluetooth.Address{}
	// Parse MAC address
	addrStr := strings.ReplaceAll(address, ":", "")
	if len(addrStr) == 12 {
		var mac [6]byte
		for i := 0; i < 6; i++ {
			var b byte
			_, err := fmt.Sscanf(addrStr[i*2:i*2+2], "%02x", &b)
			if err != nil {
				return nil, fmt.Errorf("parse address %q: %w", address, err)
			}
			mac[5-i] = b // Reverse byte order for BLE
		}
		addr.Set(fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			mac[5], mac[4], mac[3], mac[2], mac[1], mac[0]))
	} else {
		addr.Set(address)
	}

	device, err := a.adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}

	// Discover Plejd service
	serviceUUID, _ := bluetooth.ParseUUID(ble.PlejdServiceUUID)
	services, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
	if err != nil {
		device.Disconnect()
		return nil, fmt.Errorf("discover services: %w", err)
	}
	if len(services) == 0 {
		device.Disconnect()
		return nil, fmt.Errorf("plejd service not found")
	}

	// Discover all characteristics
	chars, err := services[0].DiscoverCharacteristics(nil)
	if err != nil {
		device.Disconnect()
		return nil, fmt.Errorf("discover characteristics: %w", err)
	}

	charMap := make(map[string]bluetooth.DeviceCharacteristic)
	for _, c := range chars {
		charMap[c.UUID().String()] = c
	}

	return &realDevice{device: device, chars: charMap, adapter: a.adapter}, nil
}

func (d *realDevice) WriteCharacteristic(uuid string, data []byte) error {
	c, ok := d.chars[uuid]
	if !ok {
		return fmt.Errorf("characteristic %s not found", uuid)
	}
	_, err := c.WriteWithoutResponse(data)
	return err
}

func (d *realDevice) WriteCharacteristicWithResponse(uuid string, data []byte) error {
	c, ok := d.chars[uuid]
	if !ok {
		return fmt.Errorf("characteristic %s not found", uuid)
	}
	_, err := c.Write(data)
	return err
}

func (d *realDevice) ReadCharacteristic(uuid string) ([]byte, error) {
	c, ok := d.chars[uuid]
	if !ok {
		return nil, fmt.Errorf("characteristic %s not found", uuid)
	}
	buf := make([]byte, 64)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (d *realDevice) Subscribe(uuid string, callback func([]byte)) error {
	c, ok := d.chars[uuid]
	if !ok {
		return fmt.Errorf("characteristic %s not found", uuid)
	}

	return c.EnableNotifications(func(buf []byte) {
		data := make([]byte, len(buf))
		copy(data, buf)
		callback(data)
	})
}

func (d *realDevice) Disconnect() error {
	err := d.device.Disconnect()
	// CancelConnect is non-blocking on macOS — CoreBluetooth needs time to
	// actually tear down the connection. Without this wait, the BT stack can
	// be left in a bad state, causing "timeout enabling CentralManager" on
	// the next invocation.
	time.Sleep(500 * time.Millisecond)
	return err
}
