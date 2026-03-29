package ble_test

import (
	"context"
	"testing"
	"time"

	"github.com/gloriaoracle/justplejd-cli/internal/testutil"
	"github.com/gloriaoracle/justplejd-cli/pkg/ble"
	"github.com/gloriaoracle/justplejd-cli/pkg/crypto"
	"github.com/gloriaoracle/justplejd-cli/pkg/protocol"
)

const testCryptoKey = "0123456789ABCDEF0123456789ABCDEF"
const testMAC = "AA:BB:CC:DD:EE:FF"

func setupMock() (*testutil.MockDevice, *testutil.MockAdapter) {
	device := testutil.NewMockDevice()
	// Set up auth: non-zero 16-byte challenge (poll loop skips all-zeros)
	challenge := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	device.SetCharacteristic(ble.AuthUUID, challenge)
	// Auth hook: if 1-byte init write, keep returning the challenge; otherwise store the response
	device.OnWrite(ble.AuthUUID, func(data []byte) []byte {
		if len(data) == 1 && data[0] == 0x00 {
			return challenge // keep the challenge readable
		}
		return data // store auth response
	})
	// Set up ping response: if we write X, read back (X+1)&0xFF
	device.SetCharacteristic(ble.PingUUID, []byte{0x01})
	device.OnWrite(ble.PingUUID, func(data []byte) []byte {
		return []byte{(data[0] + 1) & 0xFF}
	})

	// Build manufacturer data: company ID 887 (0x0377), MAC in bytes [4:10] reversed
	// MAC "AA:BB:CC:DD:EE:FF" reversed for storage = FF EE DD CC BB AA
	mfrData := map[uint16][]byte{
		887: {0x00, 0x00, 0x00, 0x00, 0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA},
	}

	adapter := testutil.NewMockAdapter(device, ble.ScanResult{
		Address:          "device-addr-1",
		Name:             "P mesh AABB",
		RSSI:             -50,
		ManufacturerData: mfrData,
	})

	return device, adapter
}

func TestScanForDevices(t *testing.T) {
	_, adapter := setupMock()
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	// The MAC extracted from manufacturer data should be "AA:BB:CC:DD:EE:FF"
	// DeviceID in site config is MAC without colons: "AABBCCDDEEFF"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	devices, err := p.ScanForDevices(ctx, []string{"AABBCCDDEEFF"})
	if err != nil {
		t.Fatalf("ScanForDevices failed: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].MACAddress != testMAC {
		t.Errorf("MAC: got %q, want %q", devices[0].MACAddress, testMAC)
	}
	if devices[0].RSSI != -50 {
		t.Errorf("RSSI: got %d, want -50", devices[0].RSSI)
	}
}

func TestScanFiltersNonPlejd(t *testing.T) {
	device := testutil.NewMockDevice()
	adapter := testutil.NewMockAdapter(device,
		ble.ScanResult{
			Address: "addr-1",
			Name:    "Some Other Device",
			RSSI:    -30,
		},
		ble.ScanResult{
			Address:          "addr-2",
			Name:             "P mesh AABB",
			RSSI:             -50,
			ManufacturerData: map[uint16][]byte{887: {0x00, 0x00, 0x00, 0x00, 0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA}},
		},
	)
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	devices, err := p.ScanForDevices(ctx, []string{"AABBCCDDEEFF"})
	if err != nil {
		t.Fatalf("ScanForDevices failed: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 Plejd device, got %d", len(devices))
	}
}

func TestAuthenticate(t *testing.T) {
	device, adapter := setupMock()
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	// Connect first
	err := p.Connect(ble.PlejdDevice{Address: "device-addr-1", MACAddress: testMAC, RSSI: -50})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Set up ping: after authenticate, ping writes a byte, we need to return byte+1
	// The mock needs to handle this dynamically
	// For now, just verify authenticate doesn't error
	// The mock device has auth challenge set to 16 zero bytes

	// After Authenticate, verify writes to AUTH characteristic
	authWrites := device.GetWrites(ble.AuthUUID)
	if len(authWrites) < 2 {
		t.Fatalf("expected at least 2 writes to AUTH (init + response), got %d", len(authWrites))
	}

	// First write should be 0x00
	if len(authWrites[0]) != 1 || authWrites[0][0] != 0x00 {
		t.Errorf("first auth write: got %x, want [00]", authWrites[0])
	}

	// Second write should be the auth response (16 bytes)
	if len(authWrites[1]) != 16 {
		t.Errorf("auth response length: got %d, want 16", len(authWrites[1]))
	}

	// Verify the response matches our crypto implementation
	challenge := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	expected := crypto.AuthResponse(testCryptoKey, challenge)
	for i, b := range authWrites[1] {
		if b != expected[i] {
			t.Errorf("auth response byte %d: got %02x, want %02x", i, b, expected[i])
		}
	}
}

func TestSendCommand(t *testing.T) {
	device, adapter := setupMock()
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	err := p.Connect(ble.PlejdDevice{Address: "device-addr-1", MACAddress: testMAC, RSSI: -50})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send a turn-on command
	payload := []byte{0x27, 0x01, 0x10, 0x00, 0x97, 0x01}
	err = p.SendCommand(payload)
	if err != nil {
		t.Fatalf("SendCommand failed: %v", err)
	}

	// Verify data was written to DATA_WRITE characteristic
	dataWrites := device.GetWrites(ble.DataWriteUUID)
	if len(dataWrites) == 0 {
		t.Fatal("expected write to DATA characteristic")
	}

	// The written data should be encrypted (not equal to plaintext)
	written := dataWrites[len(dataWrites)-1]
	if len(written) != len(payload) {
		t.Fatalf("written length %d != payload length %d", len(written), len(payload))
	}

	// Decrypt and verify it matches the original payload
	decrypted := crypto.EncryptDecrypt(testCryptoKey, testMAC, written)
	for i, b := range decrypted {
		if b != payload[i] {
			t.Errorf("decrypted byte %d: got %02x, want %02x", i, b, payload[i])
		}
	}
}

func TestPing(t *testing.T) {
	_, adapter := setupMock()

	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	err := p.Connect(ble.PlejdDevice{Address: "device-addr-1", MACAddress: testMAC, RSSI: -50})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	ok, err := p.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if !ok {
		t.Error("expected ping to succeed")
	}
}

func TestDisconnect(t *testing.T) {
	device, adapter := setupMock()
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	err := p.Connect(ble.PlejdDevice{Address: "device-addr-1", MACAddress: testMAC, RSSI: -50})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	err = p.Disconnect()
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	if !device.Disconnected {
		t.Error("expected device to be disconnected")
	}
}

func TestPollLightLevel(t *testing.T) {
	device, adapter := setupMock()
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	// Set up LIGHTLEVEL: write hook returns device state data after trigger write
	lightLevelData := []byte{0x05, 0x01, 0x00, 0x00, 0x00, 0xFF, 0x7F, 0x00, 0x00, 0x00}
	device.SetCharacteristic(ble.LightLevelUUID, lightLevelData)
	device.OnWrite(ble.LightLevelUUID, func(data []byte) []byte {
		return lightLevelData
	})

	err := p.Connect(ble.PlejdDevice{Address: "device-addr-1", MACAddress: testMAC, RSSI: -50})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	states, err := p.PollLightLevel()
	if err != nil {
		t.Fatalf("PollLightLevel failed: %v", err)
	}

	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].Address != 5 {
		t.Errorf("address: got %d, want 5", states[0].Address)
	}
	if states[0].State != 1 {
		t.Errorf("state: got %d, want 1", states[0].State)
	}
	if states[0].DimLevel != 0x7FFF {
		t.Errorf("dim: got %d, want %d", states[0].DimLevel, 0x7FFF)
	}

	// Verify write to LIGHTLEVEL
	writes := device.GetWrites(ble.LightLevelUUID)
	found := false
	for _, w := range writes {
		if len(w) == 1 && w[0] == 0x01 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected write of 0x01 to LIGHTLEVEL")
	}
}

func TestRequestButtonEvents(t *testing.T) {
	device, adapter := setupMock()
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	err := p.Connect(ble.PlejdDevice{Address: "device-addr-1", MACAddress: testMAC, RSSI: -50})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	err = p.RequestButtonEvents()
	if err != nil {
		t.Fatalf("RequestButtonEvents failed: %v", err)
	}

	// Verify that an encrypted command was written to DATA_WRITE
	writes := device.GetWrites(ble.DataWriteUUID)
	if len(writes) == 0 {
		t.Error("expected write to DATA characteristic for button subscribe")
	}

	// Decrypt the last write and verify it matches ButtonEventSubscribe
	lastWrite := writes[len(writes)-1]
	decrypted := crypto.EncryptDecrypt(testCryptoKey, testMAC, lastWrite)
	expected := []byte{0x00, 0x01, 0x10, 0x00, 0x15}
	if len(decrypted) != len(expected) {
		t.Fatalf("decrypted length %d != expected length %d", len(decrypted), len(expected))
	}
	for i, b := range decrypted {
		if b != expected[i] {
			t.Errorf("byte %d: got %02x, want %02x", i, b, expected[i])
		}
	}
}

func TestSubscribeLightLevel(t *testing.T) {
	device, adapter := setupMock()
	p := ble.NewPlejdBLE(adapter, testCryptoKey)

	err := p.Connect(ble.PlejdDevice{Address: "device-addr-1", MACAddress: testMAC, RSSI: -50})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	var received int
	err = p.SubscribeLightLevel(func(states []protocol.DeviceState) {
		received += len(states)
	})
	if err != nil {
		t.Fatalf("SubscribeLightLevel failed: %v", err)
	}

	// Inject a LIGHTLEVEL notification with 2 device records
	data := []byte{
		0x05, 0x01, 0x00, 0x00, 0x00, 0xFF, 0x7F, 0x00, 0x00, 0x00,
		0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	device.InjectNotification(ble.LightLevelUUID, data)

	if received != 2 {
		t.Errorf("received %d states, want 2", received)
	}
}
