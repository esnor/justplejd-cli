package protocol

// EventType identifies the type of incoming mesh event.
type EventType string

const (
	EventSceneActivated  EventType = "scene_activated"
	EventButtonPress     EventType = "button_press"
	EventDim             EventType = "dim"
	EventStateChange     EventType = "change_state"
	EventColorTemp       EventType = "color_temperature"
	EventMotion          EventType = "motion"
	EventThermostatState EventType = "thermostat_state"
	EventCoverState      EventType = "cover_state"
	EventTimeResponse    EventType = "time_response"
	EventThermostatSet   EventType = "thermostat_setpoint"
)

// ThermostatModeName maps mode number to human-readable name.
var ThermostatModeName = map[int]string{
	0: "service",
	1: "curing",
	2: "vacation",
	3: "boost",
	4: "frost",
	5: "night",
	6: "day",
	7: "normal",
}

// Event represents a parsed incoming Plejd mesh packet.
type Event struct {
	Type        EventType
	Scene       int    // scene index (scene_activated)
	Triggered   bool   // scene was triggered (scene_activated)
	Address     int    // device address
	Button      int    // button number (button_press)
	Action      string // "press" or "release" (button_press)
	State       int    // on/off state (dim, change_state)
	Dim         int    // dim level 0-255 (dim)
	Temperature int    // color temperature (color_temperature)
	LightLevel  int    // ambient light level (motion)

	// Thermostat fields
	ThermoMode    int     // thermostat mode (0-7)
	ThermoError   bool    // thermostat error flag
	ThermoTarget  float64 // target temperature °C
	ThermoCurrent float64 // current temperature °C
	ThermoHeating bool    // currently heating

	// Cover fields
	CoverPosition  int  // cover position 0-100%
	CoverDirection int  // 0=down, 1=up
	CoverTilt      int  // tilt angle (6-bit signed, 5° increments)
	CoverMoving    bool // cover is moving

	// Time field
	Timestamp int64 // Unix timestamp (time_response)
}

// ParseIncomingPacket parses a decrypted incoming BLE notification into an Event.
// Returns nil if the packet is not recognized or too short.
func ParseIncomingPacket(data []byte) *Event {
	if len(data) < 6 {
		return nil
	}

	// Scene activated: 00 0110 0021 SS
	if data[0] == 0x00 && data[1] == 0x01 && data[2] == 0x10 &&
		data[3] == 0x00 && data[4] == 0x21 {
		return &Event{
			Type:      EventSceneActivated,
			Scene:     int(data[5]),
			Triggered: true,
		}
	}

	// Button press: 00 0110 0016 AA BB [xx]
	if data[0] == 0x00 && data[1] == 0x01 && data[2] == 0x10 &&
		data[3] == 0x00 && data[4] == 0x16 && len(data) >= 7 {
		action := "press"
		if len(data) >= 8 && data[7] == 0x00 {
			action = "release"
		}
		return &Event{
			Type:    EventButtonPress,
			Address: int(data[5]),
			Button:  int(data[6]),
			Action:  action,
		}
	}

	// Time response: 01 0110 001B TTTTTTTT 01
	if data[1] == 0x01 && data[2] == 0x10 &&
		data[3] == 0x00 && data[4] == 0x1B && len(data) >= 9 {
		ts := int64(data[5]) | int64(data[6])<<8 | int64(data[7])<<16 | int64(data[8])<<24
		return &Event{
			Type:      EventTimeResponse,
			Address:   int(data[0]),
			Timestamp: ts,
		}
	}

	// Thermostat setpoint event: AA 0110 045C ...
	if data[1] == 0x01 && data[2] == 0x10 &&
		data[3] == 0x04 && data[4] == 0x5C && len(data) >= 7 {
		raw := uint16(data[5]) | uint16(data[6])<<8 // little-endian
		return &Event{
			Type:         EventThermostatSet,
			Address:      int(data[0]),
			ThermoTarget: float64(raw) / 10.0,
		}
	}

	// Dim state change: AA 0110 00C8 SS DD DD or AA 0110 0098 SS DD DD
	if data[1] == 0x01 && data[2] == 0x10 && data[3] == 0x00 &&
		(data[4] == 0xC8 || data[4] == 0x98) && len(data) >= 8 {
		return parseDimOrThermostatOrCover(data)
	}

	// On/Off state change: AA 0110 0097 SS
	if data[1] == 0x01 && data[2] == 0x10 && data[3] == 0x00 && data[4] == 0x97 {
		return &Event{
			Type:    EventStateChange,
			Address: int(data[0]),
			State:   int(data[5]),
		}
	}

	// OUTPUT_SET (0x0420): color temp, cover, motion — parse with MiniPkg awareness
	if len(data) >= 7 &&
		data[1] == 0x01 && data[2] == 0x10 && data[3] == 0x04 && data[4] == 0x20 {
		return parseOutputSet(data)
	}

	return nil
}

// parseDimOrThermostatOrCover handles 0x0098/0x00C8 packets which may encode
// light dim, thermostat state, or cover state depending on the device.
func parseDimOrThermostatOrCover(data []byte) *Event {
	// Standard dim event
	return &Event{
		Type:    EventDim,
		Address: int(data[0]),
		State:   int(data[5]),
		Dim:     int(data[7]),
	}
}

// ParseThermostatState parses the 24-bit packed thermostat state from dim bytes.
// Bits: 0000 000S MMME TTTT TTCC CCCC
//
//	S = State bit, M = Mode (3 bits), E = Error flag
//	T = Target temperature (7 bits, value - 10)
//	C = Current temperature (6 bits, value - 10)
//
// Extra byte bit 7: 1 = currently heating
func ParseThermostatState(state byte, dim1 byte, dim2 byte, extra byte) *Event {
	packed := uint32(state)<<16 | uint32(dim1)<<8 | uint32(dim2)

	// Bit layout (24 bits, MSB first):
	//   [23:18] padding
	//   [17]    S = state
	//   [16:14] MMM = mode (3 bits)
	//   [13]    E = error flag
	//   [12:6]  TTTTTTT = target temperature (7 bits, + 10 = °C)
	//   [5:0]   CCCCCC = current temperature (6 bits, + 10 = °C)
	stateVal := int((packed >> 17) & 0x01)
	mode := int((packed >> 14) & 0x07)
	errorFlag := (packed>>13)&0x01 != 0
	target := int((packed >> 6) & 0x7F)
	current := int(packed & 0x3F)
	heating := extra&0x80 != 0

	return &Event{
		Type:          EventThermostatState,
		State:         stateVal,
		ThermoMode:    mode,
		ThermoError:   errorFlag,
		ThermoTarget:  float64(target) + 10.0,
		ThermoCurrent: float64(current) + 10.0,
		ThermoHeating: heating,
	}
}

// ParseCoverState parses cover state from dim bytes.
// Position: 7-bit value (0x00-0x7F) = 0-100%
// Bit 7: direction (1=up, 0=down)
// Tilt: 6-bit signed in 5° increments
func ParseCoverState(state byte, positionByte byte, tiltByte byte) *Event {
	moving := state != 0
	position := int(positionByte & 0x7F)
	direction := int((positionByte >> 7) & 0x01)

	// Tilt is 6-bit signed
	tilt := int(tiltByte & 0x3F)
	if tiltByte&0x20 != 0 { // sign bit
		tilt = tilt - 64
	}

	return &Event{
		Type:           EventCoverState,
		CoverMoving:    moving,
		CoverPosition:  position,
		CoverDirection: direction,
		CoverTilt:      tilt,
	}
}

// parseOutputSet handles 0x0420 OUTPUT_SET events, trying MiniPkg decoding first,
// then falling back to legacy format parsing.
func parseOutputSet(data []byte) *Event {
	payload := data[5:]
	addr := int(data[0])

	// Try MiniPkg decoding: first byte should have 0x80 flag set
	if len(payload) > 0 && payload[0]&0x80 != 0 {
		pkgs, _ := DecodeMiniPkgs(payload)
		if len(pkgs) > 0 {
			return parseMiniPkgEvent(addr, pkgs)
		}
	}

	// Legacy format: 03 01 11 TT TT (color temp)
	if len(payload) >= 4 && payload[0] == 0x01 && payload[1] == 0x11 {
		temp := int(payload[2])<<8 | int(payload[3])
		return &Event{
			Type:        EventColorTemp,
			Address:     addr,
			Temperature: temp,
		}
	}

	// Legacy motion: 03 xx ... LL LL
	if len(payload) >= 2 && payload[0] == 0x03 {
		lightLevel := int(payload[len(payload)-2])<<8 | int(payload[len(payload)-1])
		return &Event{
			Type:       EventMotion,
			Address:    addr,
			LightLevel: lightLevel,
		}
	}

	return nil
}

// parseMiniPkgEvent interprets decoded MiniPkg sub-packets into an Event.
func parseMiniPkgEvent(addr int, pkgs []MiniPkg) *Event {
	ev := &Event{Address: addr}

	for _, pkg := range pkgs {
		switch pkg.Type {
		case MiniPkgWhiteBalance:
			if len(pkg.Payload) >= 2 {
				ev.Type = EventColorTemp
				ev.Temperature = int(pkg.Payload[0])<<8 | int(pkg.Payload[1])
			}
		case MiniPkgLux:
			if len(pkg.Payload) >= 1 {
				ev.Type = EventMotion
				ev.LightLevel = int(pkg.Payload[0])
			}
		case MiniPkgWindowCtrl:
			ev.Type = EventCoverState
			if len(pkg.Payload) >= 3 {
				ev.CoverMoving = pkg.Payload[0] != 0
				ev.CoverPosition = int(pkg.Payload[1])<<8 | int(pkg.Payload[2])
			} else if len(pkg.Payload) >= 1 {
				ev.CoverMoving = false // stop
			}
		case MiniPkgTilt:
			if len(pkg.Payload) >= 1 {
				ev.CoverTilt = int(pkg.Payload[0])
			}
		case MiniPkgBatteryInfo:
			// Included in motion events; don't override type
		case MiniPkgSource:
			// Source tag; informational
		}
	}

	if ev.Type == "" {
		return nil
	}
	return ev
}

// DeviceState represents a device state from LIGHTLEVEL polling.
type DeviceState struct {
	Address  int
	State    int // on/off
	DimLevel int // 0-65535
	RawExtra []byte
}

// ParseLightLevel parses LIGHTLEVEL notification data into device states.
// Each device record is 10 bytes:
//
//	Byte 0: Device address
//	Byte 1: State (on/off)
//	Bytes 2-4: Unknown
//	Bytes 5-6: Dim level (little-endian, 0-65535)
//	Bytes 7-9: Remaining (varies by type)
func ParseLightLevel(data []byte) []DeviceState {
	var states []DeviceState
	for i := 0; i+10 <= len(data); i += 10 {
		record := data[i : i+10]
		dimLevel := int(record[5]) | int(record[6])<<8
		states = append(states, DeviceState{
			Address:  int(record[0]),
			State:    int(record[1]),
			DimLevel: dimLevel,
			RawExtra: record[7:10],
		})
	}
	return states
}
