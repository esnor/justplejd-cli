// Package ble provides BLE communication with Plejd mesh devices.
package ble

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gloriaoracle/justplejd-cli/pkg/crypto"
	"github.com/gloriaoracle/justplejd-cli/pkg/protocol"
)

// BLE UUIDs for Plejd service and characteristics.
const (
	PlejdServiceUUID = "31ba0001-6085-4726-be45-040c957391b5"
	LightLevelUUID   = "31ba0003-6085-4726-be45-040c957391b5"
	DataWriteUUID    = "31ba0004-6085-4726-be45-040c957391b5"
	DataNotifyUUID   = "31ba0005-6085-4726-be45-040c957391b5"
	AuthUUID         = "31ba0009-6085-4726-be45-040c957391b5"
	PingUUID         = "31ba000a-6085-4726-be45-040c957391b5"
)

// ScanResult represents a discovered BLE device.
type ScanResult struct {
	Address          string
	Name             string
	RSSI             int
	ManufacturerData map[uint16][]byte
}

// Device represents a connected BLE GATT device.
type Device interface {
	WriteCharacteristic(uuid string, data []byte) error
	WriteCharacteristicWithResponse(uuid string, data []byte) error
	ReadCharacteristic(uuid string) ([]byte, error)
	Subscribe(uuid string, callback func([]byte)) error
	Disconnect() error
}

// Adapter is the interface for BLE hardware operations.
type Adapter interface {
	Scan(ctx context.Context, callback func(ScanResult)) error
	StopScan() error
	Connect(address string) (Device, error)
}

// PlejdDevice represents a discovered Plejd mesh device.
type PlejdDevice struct {
	Address    string
	MACAddress string
	RSSI       int
}

// PlejdBLE manages BLE communication with a Plejd mesh gateway.
type PlejdBLE struct {
	adapter    Adapter
	cryptoKey  string
	device     Device
	macAddress string
}

// NewPlejdBLE creates a new PlejdBLE instance.
func NewPlejdBLE(adapter Adapter, cryptoKey string) *PlejdBLE {
	return &PlejdBLE{adapter: adapter, cryptoKey: cryptoKey}
}

// ScanForDevices scans for Plejd devices matching known device IDs.
// Returns devices sorted by RSSI (strongest first).
func (p *PlejdBLE) ScanForDevices(ctx context.Context, knownDeviceIDs []string) ([]PlejdDevice, error) {
	known := make(map[string]bool, len(knownDeviceIDs))
	for _, id := range knownDeviceIDs {
		known[strings.ToUpper(id)] = true
	}

	var devices []PlejdDevice

	err := p.adapter.Scan(ctx, func(r ScanResult) {
		if !strings.HasPrefix(r.Name, "P mesh") {
			return
		}

		mfrData, ok := r.ManufacturerData[887]
		if !ok {
			return
		}

		mac := crypto.ExtractMACAddress(mfrData)
		if mac == "" {
			return
		}

		macNoColon := strings.ReplaceAll(mac, ":", "")
		if !known[strings.ToUpper(macNoColon)] {
			return
		}

		devices = append(devices, PlejdDevice{
			Address:    r.Address,
			MACAddress: mac,
			RSSI:       r.RSSI,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].RSSI > devices[j].RSSI
	})

	return devices, nil
}

// Connect connects to a Plejd device and authenticates.
func (p *PlejdBLE) Connect(device PlejdDevice) error {
	dev, err := p.adapter.Connect(device.Address)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	p.device = dev
	p.macAddress = device.MACAddress

	if err := p.Authenticate(); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}

	return nil
}

// Authenticate performs the Plejd BLE auth handshake.
func (p *PlejdBLE) Authenticate() error {
	// Step 1: Write 0x00 to AUTH characteristic to request a challenge
	if err := p.device.WriteCharacteristicWithResponse(AuthUUID, []byte{0x00}); err != nil {
		return fmt.Errorf("write auth init: %w", err)
	}

	// Step 2: Poll for the challenge — the device generates a 16-byte nonce
	// after receiving the init write. Poll-read until we get a valid challenge
	// instead of using an arbitrary sleep.
	var challenge []byte
	for attempt := 0; attempt < 20; attempt++ {
		time.Sleep(50 * time.Millisecond)
		data, err := p.device.ReadCharacteristic(AuthUUID)
		if err != nil {
			continue
		}
		if len(data) == 16 && !isAllZeros(data) {
			challenge = data
			break
		}
	}
	if challenge == nil {
		return fmt.Errorf("timed out waiting for auth challenge")
	}

	// Step 3: Compute and write response
	response := crypto.AuthResponse(p.cryptoKey, challenge)
	if response == nil {
		return fmt.Errorf("failed to compute auth response")
	}

	if err := p.device.WriteCharacteristicWithResponse(AuthUUID, response); err != nil {
		return fmt.Errorf("write auth response: %w", err)
	}

	return nil
}

// isAllZeros returns true if every byte in data is 0x00.
func isAllZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

// Ping sends a ping and verifies the pong response.
func (p *PlejdBLE) Ping() (bool, error) {
	ping := make([]byte, 1)
	rand.Read(ping)
	

	if err := p.device.WriteCharacteristicWithResponse(PingUUID, ping); err != nil {
		return false, fmt.Errorf("write ping: %w", err)
	}

	pong, err := p.device.ReadCharacteristic(PingUUID)
	if err != nil {
		return false, fmt.Errorf("read pong: %w", err)
	}

	if len(pong) == 0 {
		return false, nil
	}

	expected := (ping[0] + 1) & 0xFF
	
	return pong[0] == expected, nil
}

// SendCommand encrypts and sends a command payload.
func (p *PlejdBLE) SendCommand(payload []byte) error {
	encrypted := crypto.EncryptDecrypt(p.cryptoKey, p.macAddress, payload)
	if encrypted == nil {
		return fmt.Errorf("encryption failed")
	}

	if err := p.device.WriteCharacteristicWithResponse(DataWriteUUID, encrypted); err != nil {
		return fmt.Errorf("write command: %w", err)
	}

	return nil
}

// Listen subscribes to data notifications and calls the callback with parsed events.
func (p *PlejdBLE) Listen(callback func(protocol.Event)) error {
	return p.device.Subscribe(DataNotifyUUID, func(data []byte) {
		decrypted := crypto.EncryptDecrypt(p.cryptoKey, p.macAddress, data)
		if decrypted == nil {
			return
		}
		event := protocol.ParseIncomingPacket(decrypted)
		if event == nil {
			return
		}
		callback(*event)
	})
}

// PollLightLevel writes 0x01 to the LIGHTLEVEL characteristic to trigger
// state reports, then reads the response and parses device state records.
func (p *PlejdBLE) PollLightLevel() ([]protocol.DeviceState, error) {
	if err := p.device.WriteCharacteristicWithResponse(LightLevelUUID, []byte{0x01}); err != nil {
		return nil, fmt.Errorf("write lightlevel: %w", err)
	}

	data, err := p.device.ReadCharacteristic(LightLevelUUID)
	if err != nil {
		return nil, fmt.Errorf("read lightlevel: %w", err)
	}

	return protocol.ParseLightLevel(data), nil
}

// SubscribeLightLevel subscribes to LIGHTLEVEL notifications and calls the
// callback with parsed device states.
func (p *PlejdBLE) SubscribeLightLevel(callback func([]protocol.DeviceState)) error {
	return p.device.Subscribe(LightLevelUUID, func(data []byte) {
		states := protocol.ParseLightLevel(data)
		if len(states) > 0 {
			callback(states)
		}
	})
}

// RequestButtonEvents sends CMD_EVENT_PREPARE (0x0015) to enable all button events.
func (p *PlejdBLE) RequestButtonEvents() error {
	payload := protocol.ButtonEventSubscribe()
	return p.SendCommand(payload)
}

// Disconnect disconnects from the Plejd device.
func (p *PlejdBLE) Disconnect() error {
	if p.device == nil {
		return nil
	}
	return p.device.Disconnect()
}
