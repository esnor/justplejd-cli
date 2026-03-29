package protocol

import (
	"encoding/hex"
	"testing"
)

func assertHex(t *testing.T, name string, got []byte, wantHex string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: got nil", name)
	}
	gotHex := hex.EncodeToString(got)
	if gotHex != wantHex {
		t.Errorf("%s:\n  got  %s\n  want %s", name, gotHex, wantHex)
	}
}

func TestTurnOn(t *testing.T) {
	// Address 0x27: 27 0110 0097 01
	result := TurnOn(0x27)
	assertHex(t, "TurnOn(0x27)", result, "270110009701")

	// Address 0x01
	result = TurnOn(0x01)
	assertHex(t, "TurnOn(0x01)", result, "010110009701")
}

func TestTurnOff(t *testing.T) {
	// Address 0x27: 27 0110 0097 00
	result := TurnOff(0x27)
	assertHex(t, "TurnOff(0x27)", result, "270110009700")

	// Address 0x01
	result = TurnOff(0x01)
	assertHex(t, "TurnOff(0x01)", result, "010110009700")
}

func TestDim(t *testing.T) {
	// Address 0x27, level 182 (0xB6): 27 0110 0098 01 B6B6
	result := Dim(0x27, 0xB6)
	assertHex(t, "Dim(0x27, 0xB6)", result, "270110009801b6b6")

	// Full brightness (255)
	result = Dim(0x27, 0xFF)
	assertHex(t, "Dim(0x27, 0xFF)", result, "270110009801ffff")

	// Off (0)
	result = Dim(0x27, 0x00)
	assertHex(t, "Dim(0x27, 0x00)", result, "2701100098010000")
}

func TestColorTemperature(t *testing.T) {
	// Address 0x27, temp 400 (0x0190): 27 0110 0420 030111 0190
	result := ColorTemperature(0x27, 0x0190)
	assertHex(t, "ColorTemp(0x27, 400)", result, "27011004200301110190")
}

func TestCover(t *testing.T) {
	// Address 0x27, position 100 (0x0064): 27 0110 0420 030827 01 0064
	result := Cover(0x27, 0x0064)
	assertHex(t, "Cover(0x27, 100)", result, "2701100420030827010064")
}

func TestCoverStop(t *testing.T) {
	// Address 0x27: 27 0110 0420 030807 00
	result := CoverStop(0x27)
	assertHex(t, "CoverStop(0x27)", result, "270110042003080700")
}

func TestActivateScene(t *testing.T) {
	// Scene index 1: 02 0110 0021 01
	result := ActivateScene(0x01)
	assertHex(t, "Scene(1)", result, "020110002101")

	// Scene index 5
	result = ActivateScene(0x05)
	assertHex(t, "Scene(5)", result, "020110002105")
}

func TestSetTime(t *testing.T) {
	// Timestamp 1700000000 = 0x6553F100
	// 5 bytes LE: 00 F1 53 65 00
	// Full command: 00 0110 001B 00F1536500
	result := SetTime(1700000000)
	assertHex(t, "SetTime(1700000000)", result, "000110001b00f1536500")
}

func TestRequestTime(t *testing.T) {
	// Address 0x27: 27 0102 001B
	result := RequestTime(0x27)
	assertHex(t, "RequestTime(0x27)", result, "270102001b")
}

func TestThermostatSetTemp(t *testing.T) {
	// 22.5°C = 225 = 0x00E1, little-endian: E1 00
	// Full: 27 0110 045C E100
	result := ThermostatSetTemp(0x27, 22.5)
	assertHex(t, "ThermostatSetTemp(22.5)", result, "270110045ce100")

	// 18.0°C = 180 = 0x00B4, little-endian: B4 00
	result = ThermostatSetTemp(0x27, 18.0)
	assertHex(t, "ThermostatSetTemp(18.0)", result, "270110045cb400")
}

func TestThermostatMode(t *testing.T) {
	// Normal mode (7): 27 0110 045F 07
	result := ThermostatMode(0x27, 7)
	assertHex(t, "ThermostatMode(normal)", result, "270110045f07")

	// Boost mode (3): 27 0110 045F 03
	result = ThermostatMode(0x27, 3)
	assertHex(t, "ThermostatMode(boost)", result, "270110045f03")
}

func TestThermostatPWM(t *testing.T) {
	// 50% duty: 27 0110 0461 32
	result := ThermostatPWM(0x27, 50)
	assertHex(t, "ThermostatPWM(50)", result, "270110046132")

	// 100% duty: 27 0110 0461 64
	result = ThermostatPWM(0x27, 100)
	assertHex(t, "ThermostatPWM(100)", result, "270110046164")
}

func TestThermostatReset(t *testing.T) {
	// 27 0110 047E
	result := ThermostatReset(0x27)
	assertHex(t, "ThermostatReset", result, "270110047e")
}

func TestButtonEventSubscribe(t *testing.T) {
	// 00 0110 0015
	result := ButtonEventSubscribe()
	assertHex(t, "ButtonEventSubscribe", result, "0001100015")
}

func TestAmbientLightRequest(t *testing.T) {
	// 27 0102 0434
	result := AmbientLightRequest(0x27)
	assertHex(t, "AmbientLightRequest(0x27)", result, "2701020434")
}

func TestColorTemperatureMiniPkg(t *testing.T) {
	result := ColorTemperatureMiniPkg(0x27, 3000)
	if result == nil {
		t.Fatal("got nil")
	}
	// Header: 27 0110 0420
	// Source(APP): 83 08
	// WhiteBalance(3000=0x0BB8): 91 0B B8
	assertHex(t, "ColorTempMiniPkg(3000)", result, "27011004208308910bb8")
}

func TestCoverMiniPkg(t *testing.T) {
	result := CoverMiniPkg(0x27, 0x0064)
	if result == nil {
		t.Fatal("got nil")
	}
	// Header: 27 0110 0420
	// Source(APP): 83 08
	// WindowControl(set, 0x0064): A7 01 00 64
	assertHex(t, "CoverMiniPkg(100)", result, "27011004208308a7010064")
}

func TestCoverStopMiniPkg(t *testing.T) {
	result := CoverStopMiniPkg(0x27)
	if result == nil {
		t.Fatal("got nil")
	}
	// Header: 27 0110 0420
	// Source(APP): 83 08
	// WindowControl(stop): 87 00
	assertHex(t, "CoverStopMiniPkg", result, "270110042083088700")
}

func TestCoverTilt(t *testing.T) {
	result := CoverTilt(0x27, 45)
	if result == nil {
		t.Fatal("got nil")
	}
	// Header: 27 0110 0420
	// Source(APP): 83 08
	// Tilt(45=0x2D): 8F 09 2D (extended type 0x18, TTTT=0x0F, ext=9)
	assertHex(t, "CoverTilt(45)", result, "270110042083088f092d")
}
