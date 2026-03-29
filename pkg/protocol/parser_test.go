package protocol

import "testing"

func TestParseSceneActivated(t *testing.T) {
	// 00 0110 0021 05 -> scene 5 activated
	data := []byte{0x00, 0x01, 0x10, 0x00, 0x21, 0x05}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventSceneActivated {
		t.Errorf("type: got %q, want %q", ev.Type, EventSceneActivated)
	}
	if ev.Scene != 5 {
		t.Errorf("scene: got %d, want 5", ev.Scene)
	}
	if !ev.Triggered {
		t.Error("triggered: got false, want true")
	}
}

func TestParseButtonPress(t *testing.T) {
	// 00 0110 0016 27 01 01 -> addr 0x27, button 1, press
	data := []byte{0x00, 0x01, 0x10, 0x00, 0x16, 0x27, 0x01, 0x01}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventButtonPress {
		t.Errorf("type: got %q, want %q", ev.Type, EventButtonPress)
	}
	if ev.Address != 0x27 {
		t.Errorf("address: got %d, want %d", ev.Address, 0x27)
	}
	if ev.Button != 1 {
		t.Errorf("button: got %d, want 1", ev.Button)
	}
	if ev.Action != "press" {
		t.Errorf("action: got %q, want %q", ev.Action, "press")
	}
}

func TestParseButtonRelease(t *testing.T) {
	// 00 0110 0016 27 01 00 -> addr 0x27, button 1, release
	data := []byte{0x00, 0x01, 0x10, 0x00, 0x16, 0x27, 0x01, 0x00}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventButtonPress {
		t.Errorf("type: got %q, want %q", ev.Type, EventButtonPress)
	}
	if ev.Action != "release" {
		t.Errorf("action: got %q, want %q", ev.Action, "release")
	}
}

func TestParseDimC8(t *testing.T) {
	// 27 0110 00C8 01 B6 B6 -> addr 0x27, state 1, dim 182
	data := []byte{0x27, 0x01, 0x10, 0x00, 0xC8, 0x01, 0xB6, 0xB6}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventDim {
		t.Errorf("type: got %q, want %q", ev.Type, EventDim)
	}
	if ev.Address != 0x27 {
		t.Errorf("address: got %d, want %d", ev.Address, 0x27)
	}
	if ev.State != 1 {
		t.Errorf("state: got %d, want 1", ev.State)
	}
	if ev.Dim != 0xB6 {
		t.Errorf("dim: got %d, want %d", ev.Dim, 0xB6)
	}
}

func TestParseDim98(t *testing.T) {
	// 27 0110 0098 01 B6 B6 -> addr 0x27, state 1, dim 182
	data := []byte{0x27, 0x01, 0x10, 0x00, 0x98, 0x01, 0xB6, 0xB6}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventDim {
		t.Errorf("type: got %q, want %q", ev.Type, EventDim)
	}
	if ev.Address != 0x27 {
		t.Errorf("address: got %d, want %d", ev.Address, 0x27)
	}
	if ev.Dim != 0xB6 {
		t.Errorf("dim: got %d, want %d", ev.Dim, 0xB6)
	}
}

func TestParseStateChange(t *testing.T) {
	// 27 0110 0097 01 -> addr 0x27, state on
	data := []byte{0x27, 0x01, 0x10, 0x00, 0x97, 0x01}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventStateChange {
		t.Errorf("type: got %q, want %q", ev.Type, EventStateChange)
	}
	if ev.Address != 0x27 {
		t.Errorf("address: got %d, want %d", ev.Address, 0x27)
	}
	if ev.State != 1 {
		t.Errorf("state: got %d, want 1", ev.State)
	}
}

func TestParseStateChangeOff(t *testing.T) {
	// 27 0110 0097 00 -> addr 0x27, state off
	data := []byte{0x27, 0x01, 0x10, 0x00, 0x97, 0x00}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.State != 0 {
		t.Errorf("state: got %d, want 0", ev.State)
	}
}

func TestParseColorTemperature(t *testing.T) {
	// 27 0110 0420 01 11 01 90 -> addr 0x27, temp 400
	data := []byte{0x27, 0x01, 0x10, 0x04, 0x20, 0x01, 0x11, 0x01, 0x90}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventColorTemp {
		t.Errorf("type: got %q, want %q", ev.Type, EventColorTemp)
	}
	if ev.Address != 0x27 {
		t.Errorf("address: got %d, want %d", ev.Address, 0x27)
	}
	if ev.Temperature != 400 {
		t.Errorf("temperature: got %d, want 400", ev.Temperature)
	}
}

func TestParseMotion(t *testing.T) {
	// 27 0110 0420 03 00 00 01 90 -> addr 0x27, lightlevel 400
	data := []byte{0x27, 0x01, 0x10, 0x04, 0x20, 0x03, 0x00, 0x00, 0x01, 0x90}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventMotion {
		t.Errorf("type: got %q, want %q", ev.Type, EventMotion)
	}
	if ev.Address != 0x27 {
		t.Errorf("address: got %d, want %d", ev.Address, 0x27)
	}
	if ev.LightLevel != 400 {
		t.Errorf("lightlevel: got %d, want 400", ev.LightLevel)
	}
}

func TestParseTooShort(t *testing.T) {
	data := []byte{0x00, 0x01, 0x10}
	ev := ParseIncomingPacket(data)
	if ev != nil {
		t.Errorf("expected nil for short packet, got %+v", ev)
	}
}

func TestParseEmpty(t *testing.T) {
	ev := ParseIncomingPacket([]byte{})
	if ev != nil {
		t.Errorf("expected nil for empty packet, got %+v", ev)
	}
}

func TestParseUnknownCommand(t *testing.T) {
	// Unknown command should return nil
	data := []byte{0x27, 0x01, 0x10, 0xFF, 0xFF, 0x01}
	ev := ParseIncomingPacket(data)
	if ev != nil {
		t.Errorf("expected nil for unknown command, got %+v", ev)
	}
}

func TestParseThermostatState(t *testing.T) {
	// Test packed thermostat state: mode=7(normal), no error, target=32(+10=42°C), current=22(+10=32°C)
	// Bit layout: [23:18]pad [17]S [16:14]MMM [13]E [12:6]TTTTTTT [5:0]CCCCCC
	// S=1, M=7(111), E=0, T=32(0100000), C=22(010110)
	// Binary: 000000 1 111 0 0100000 010110
	// Byte 0: 00000011 = 0x03
	// Byte 1: 11001000 = 0xC8
	// Byte 2: 00010110 = 0x16
	ev := ParseThermostatState(0x03, 0xC8, 0x16, 0x80)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventThermostatState {
		t.Errorf("type: got %q, want %q", ev.Type, EventThermostatState)
	}
	if ev.State != 1 {
		t.Errorf("state: got %d, want 1", ev.State)
	}
	if ev.ThermoMode != 7 {
		t.Errorf("mode: got %d, want 7", ev.ThermoMode)
	}
	if ev.ThermoError {
		t.Error("error: got true, want false")
	}
	if ev.ThermoTarget != 42.0 {
		t.Errorf("target: got %.1f, want 42.0", ev.ThermoTarget)
	}
	if ev.ThermoCurrent != 32.0 {
		t.Errorf("current: got %.1f, want 32.0", ev.ThermoCurrent)
	}
	if !ev.ThermoHeating {
		t.Error("heating: got false, want true")
	}
}

func TestParseThermostatStateError(t *testing.T) {
	// Mode=0(service), error flag set, target=20(+10=30), current=10(+10=20)
	// Bit layout: [23:18]pad [17]S [16:14]MMM [13]E [12:6]TTTTTTT [5:0]CCCCCC
	// S=0, M=0(000), E=1, T=20(0010100), C=10(001010)
	// Binary: 000000 0 000 1 0010100 001010
	// Byte 0: 00000000 = 0x00
	// Byte 1: 00100101 = 0x25
	// Byte 2: 00001010 = 0x0A
	ev := ParseThermostatState(0x00, 0x25, 0x0A, 0x00)
	if ev.ThermoMode != 0 {
		t.Errorf("mode: got %d, want 0", ev.ThermoMode)
	}
	if !ev.ThermoError {
		t.Error("error: got false, want true")
	}
	if ev.ThermoTarget != 30.0 {
		t.Errorf("target: got %.1f, want 30.0", ev.ThermoTarget)
	}
	if ev.ThermoCurrent != 20.0 {
		t.Errorf("current: got %.1f, want 20.0", ev.ThermoCurrent)
	}
	if ev.ThermoHeating {
		t.Error("heating: got true, want false")
	}
}

func TestParseCoverState(t *testing.T) {
	// Position 50 (0x32), direction up (bit 7 set): 0xB2
	// Tilt: 10 (positive, fits in 6 bits): 0x0A
	ev := ParseCoverState(0x01, 0xB2, 0x0A)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventCoverState {
		t.Errorf("type: got %q, want %q", ev.Type, EventCoverState)
	}
	if !ev.CoverMoving {
		t.Error("moving: got false, want true")
	}
	if ev.CoverPosition != 50 {
		t.Errorf("position: got %d, want 50", ev.CoverPosition)
	}
	if ev.CoverDirection != 1 {
		t.Errorf("direction: got %d, want 1 (up)", ev.CoverDirection)
	}
	if ev.CoverTilt != 10 {
		t.Errorf("tilt: got %d, want 10", ev.CoverTilt)
	}
}

func TestParseCoverStateStopped(t *testing.T) {
	// Stopped, position 100 (0x64), direction down (bit 7 clear)
	ev := ParseCoverState(0x00, 0x64, 0x00)
	if !ev.CoverMoving {
		// state=0 means stopped
	}
	if ev.CoverPosition != 100 {
		t.Errorf("position: got %d, want 100", ev.CoverPosition)
	}
	if ev.CoverDirection != 0 {
		t.Errorf("direction: got %d, want 0 (down)", ev.CoverDirection)
	}
}

func TestParseCoverStateNegativeTilt(t *testing.T) {
	// Tilt with sign bit set (bit 5): 0x30 = 110000 binary
	// Sign bit set, value = 48 - 64 = -16
	ev := ParseCoverState(0x00, 0x00, 0x30)
	if ev.CoverTilt != -16 {
		t.Errorf("tilt: got %d, want -16", ev.CoverTilt)
	}
}

func TestParseTimeResponse(t *testing.T) {
	// 01 0110 001B TTTTTTTT 01
	// Timestamp: 1700000000 = 0x6553F100
	// LE: 00 F1 53 65
	data := []byte{0x01, 0x01, 0x10, 0x00, 0x1B, 0x00, 0xF1, 0x53, 0x65, 0x01}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventTimeResponse {
		t.Errorf("type: got %q, want %q", ev.Type, EventTimeResponse)
	}
	if ev.Timestamp != 1700000000 {
		t.Errorf("timestamp: got %d, want 1700000000", ev.Timestamp)
	}
}

func TestParseThermostatSetpoint(t *testing.T) {
	// AA 0110 045C TTTT (little-endian)
	// 22.5°C = 225 = 0xE1, LE: E1 00
	data := []byte{0x27, 0x01, 0x10, 0x04, 0x5C, 0xE1, 0x00}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventThermostatSet {
		t.Errorf("type: got %q, want %q", ev.Type, EventThermostatSet)
	}
	if ev.Address != 0x27 {
		t.Errorf("address: got %d, want %d", ev.Address, 0x27)
	}
	if ev.ThermoTarget != 22.5 {
		t.Errorf("target: got %.1f, want 22.5", ev.ThermoTarget)
	}
}

func TestParseLightLevel(t *testing.T) {
	// Two device records (10 bytes each)
	data := []byte{
		// Device 1: addr=5, state=1, unknown(3), dim=0x7FFF(LE), extra(3)
		0x05, 0x01, 0x00, 0x00, 0x00, 0xFF, 0x7F, 0x00, 0x00, 0x00,
		// Device 2: addr=10, state=0, unknown(3), dim=0x0000(LE), extra(3)
		0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	states := ParseLightLevel(data)
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}

	if states[0].Address != 5 {
		t.Errorf("state[0] address: got %d, want 5", states[0].Address)
	}
	if states[0].State != 1 {
		t.Errorf("state[0] state: got %d, want 1", states[0].State)
	}
	if states[0].DimLevel != 0x7FFF {
		t.Errorf("state[0] dim: got %d, want %d", states[0].DimLevel, 0x7FFF)
	}

	if states[1].Address != 10 {
		t.Errorf("state[1] address: got %d, want 10", states[1].Address)
	}
	if states[1].State != 0 {
		t.Errorf("state[1] state: got %d, want 0", states[1].State)
	}
	if states[1].DimLevel != 0 {
		t.Errorf("state[1] dim: got %d, want 0", states[1].DimLevel)
	}
}

func TestParseLightLevelPartial(t *testing.T) {
	// Less than 10 bytes should return empty
	data := []byte{0x05, 0x01, 0x00, 0x00, 0x00}
	states := ParseLightLevel(data)
	if len(states) != 0 {
		t.Errorf("expected 0 states for partial data, got %d", len(states))
	}
}

func TestParseLightLevelEmpty(t *testing.T) {
	states := ParseLightLevel([]byte{})
	if len(states) != 0 {
		t.Errorf("expected 0 states for empty data, got %d", len(states))
	}
}

func TestParseOutputSetMiniPkg(t *testing.T) {
	// OUTPUT_SET with MiniPkg: addr=0x27, Source(APP) + WhiteBalance(3000)
	// 27 0110 0420 [83 08] [91 0B B8]
	data := []byte{0x27, 0x01, 0x10, 0x04, 0x20, 0x83, 0x08, 0x91, 0x0B, 0xB8}
	ev := ParseIncomingPacket(data)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != EventColorTemp {
		t.Errorf("type: got %q, want %q", ev.Type, EventColorTemp)
	}
	if ev.Temperature != 3000 {
		t.Errorf("temperature: got %d, want 3000", ev.Temperature)
	}
}
