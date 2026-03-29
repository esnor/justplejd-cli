// Package protocol implements Plejd mesh protocol command encoding and packet parsing.
package protocol

// TurnOn creates a turn-on command for the given device address.
// Format: AA 0110 0097 01
func TurnOn(addr byte) []byte {
	return []byte{addr, 0x01, 0x10, 0x00, 0x97, 0x01}
}

// TurnOff creates a turn-off command for the given device address.
// Format: AA 0110 0097 00
func TurnOff(addr byte) []byte {
	return []byte{addr, 0x01, 0x10, 0x00, 0x97, 0x00}
}

// Dim creates a dim command for the given device address and level (0-255).
// Format: AA 0110 0098 01 DDDD
func Dim(addr byte, level byte) []byte {
	return []byte{addr, 0x01, 0x10, 0x00, 0x98, 0x01, level, level}
}

// ColorTemperature creates a color temperature command.
// Format: AA 0110 0420 030111 TTTT
func ColorTemperature(addr byte, temp uint16) []byte {
	return []byte{addr, 0x01, 0x10, 0x04, 0x20, 0x03, 0x01, 0x11,
		byte(temp >> 8), byte(temp & 0xFF)}
}

// Cover creates a cover position command (0-65535).
// Format: AA 0110 0420 030827 01 PPPP
func Cover(addr byte, position uint16) []byte {
	return []byte{addr, 0x01, 0x10, 0x04, 0x20, 0x03, 0x08, 0x27, 0x01,
		byte(position >> 8), byte(position & 0xFF)}
}

// CoverStop creates a cover stop command.
// Format: AA 0110 0420 030807 00
func CoverStop(addr byte) []byte {
	return []byte{addr, 0x01, 0x10, 0x04, 0x20, 0x03, 0x08, 0x07, 0x00}
}

// ActivateScene creates a scene activation command.
// Format: 02 0110 0021 II
func ActivateScene(sceneIndex byte) []byte {
	return []byte{0x02, 0x01, 0x10, 0x00, 0x21, sceneIndex}
}

// SetTime creates a time-set command with a Unix timestamp.
// Format: 00 0110 001B TTTTTTTTTT (5 bytes LE)
func SetTime(timestamp int64) []byte {
	ts := uint64(timestamp)
	return []byte{0x00, 0x01, 0x10, 0x00, 0x1B,
		byte(ts), byte(ts >> 8), byte(ts >> 16), byte(ts >> 24), byte(ts >> 32)}
}

// RequestTime creates a time request command.
// Format: AA 0102 001B
func RequestTime(addr byte) []byte {
	return []byte{addr, 0x01, 0x02, 0x00, 0x1B}
}

// ThermostatSetTemp creates a thermostat setpoint command.
// Temperature is in °C, encoded as temp × 10, little-endian.
// Format: AA 0110 045C TTTT
func ThermostatSetTemp(addr byte, tempCelsius float64) []byte {
	raw := uint16(tempCelsius * 10)
	return []byte{addr, 0x01, 0x10, 0x04, 0x5C, byte(raw & 0xFF), byte(raw >> 8)}
}

// ThermostatMode creates a thermostat operating mode command.
// Mode: 0=service, 1=curing, 2=vacation, 3=boost, 4=frost, 5=night, 6=day, 7=normal
// Format: AA 0110 045F MM
func ThermostatMode(addr byte, mode byte) []byte {
	return []byte{addr, 0x01, 0x10, 0x04, 0x5F, mode}
}

// ThermostatPWM creates a thermostat PWM duty cycle command.
// Duty is 0-100%.
// Format: AA 0110 0461 DD
func ThermostatPWM(addr byte, duty byte) []byte {
	return []byte{addr, 0x01, 0x10, 0x04, 0x61, duty}
}

// ThermostatReset creates a thermostat reset-to-normal command.
// Format: AA 0110 047E
func ThermostatReset(addr byte) []byte {
	return []byte{addr, 0x01, 0x10, 0x04, 0x7E}
}

// ButtonEventSubscribe sends CMD_EVENT_PREPARE to enable all button events.
// Format: 00 0110 0015
func ButtonEventSubscribe() []byte {
	return []byte{0x00, 0x01, 0x10, 0x00, 0x15}
}

// AmbientLightRequest requests an ambient light reading from a motion sensor.
// Format: AA 0102 0434
func AmbientLightRequest(addr byte) []byte {
	return []byte{addr, 0x01, 0x02, 0x04, 0x34}
}

// ColorTemperatureMiniPkg creates a color temperature command using MiniPkg encoding.
// Format: AA 0110 0420 [Source=APP] [WhiteBalance=TTTT]
func ColorTemperatureMiniPkg(addr byte, kelvin uint16) []byte {
	header := []byte{addr, 0x01, 0x10, 0x04, 0x20}
	src := EncodeMiniPkg(MiniPkgSourceTag(SourceApp))
	wb := EncodeMiniPkg(MiniPkgWhiteBalanceTag(kelvin))
	result := make([]byte, 0, len(header)+len(src)+len(wb))
	result = append(result, header...)
	result = append(result, src...)
	result = append(result, wb...)
	return result
}

// CoverMiniPkg creates a cover position command using MiniPkg encoding.
// Format: AA 0110 0420 [Source=APP] [WindowControl: 01 PPPP]
func CoverMiniPkg(addr byte, position uint16) []byte {
	header := []byte{addr, 0x01, 0x10, 0x04, 0x20}
	src := EncodeMiniPkg(MiniPkgSourceTag(SourceApp))
	wc := EncodeMiniPkg(MiniPkgWindowControlSet(position))
	result := make([]byte, 0, len(header)+len(src)+len(wc))
	result = append(result, header...)
	result = append(result, src...)
	result = append(result, wc...)
	return result
}

// CoverStopMiniPkg creates a cover stop command using MiniPkg encoding.
// Format: AA 0110 0420 [Source=APP] [WindowControl: 00]
func CoverStopMiniPkg(addr byte) []byte {
	header := []byte{addr, 0x01, 0x10, 0x04, 0x20}
	src := EncodeMiniPkg(MiniPkgSourceTag(SourceApp))
	wc := EncodeMiniPkg(MiniPkgWindowControlStop())
	result := make([]byte, 0, len(header)+len(src)+len(wc))
	result = append(result, header...)
	result = append(result, src...)
	result = append(result, wc...)
	return result
}

// CoverTilt creates a cover tilt command using MiniPkg encoding.
// Format: AA 0110 0420 [Source=APP] [Tilt=AA]
func CoverTilt(addr byte, angle byte) []byte {
	header := []byte{addr, 0x01, 0x10, 0x04, 0x20}
	src := EncodeMiniPkg(MiniPkgSourceTag(SourceApp))
	tilt := EncodeMiniPkg(MiniPkgTiltTag(angle))
	result := make([]byte, 0, len(header)+len(src)+len(tilt))
	result = append(result, header...)
	result = append(result, src...)
	result = append(result, tilt...)
	return result
}
