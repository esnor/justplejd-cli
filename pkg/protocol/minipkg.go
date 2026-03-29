package protocol

import "fmt"

// MiniPkg types used in OUTPUT_SET (0x0420) sub-packets.
const (
	MiniPkgWhiteBalance = 0x01 // Color temperature (2 bytes BE Kelvin)
	MiniPkgSource       = 0x03 // Command source
	MiniPkgLux          = 0x06 // Light level (1=dark, 2=bright)
	MiniPkgWindowCtrl   = 0x07 // Cover control
	MiniPkgChannel      = 0x10 // Channel selector
	MiniPkgBatteryInfo  = 0x16 // Battery voltage (2 bytes BE)
	MiniPkgTilt         = 0x18 // Cover tilt angle (1 byte)
)

// MiniPkg source values.
const (
	SourceManual = 0x01
	SourceMotion = 0x03
	SourceApp    = 0x08
)

// MiniPkg represents a decoded sub-packet.
type MiniPkg struct {
	Type    int
	Payload []byte
}

// EncodeMiniPkg encodes a MiniPkg sub-packet into wire format.
// Header byte: FSSS TTTT
//
//	F    = 0x80 flag bit (always set)
//	SSS  = payload size - 1 (3 bits)
//	TTTT = type (4 bits, 0x0-0xE)
//
// If type >= 0x0F: TTTT = 0x0F, next byte = type - 15
func EncodeMiniPkg(pkg MiniPkg) []byte {
	size := len(pkg.Payload)
	if size < 1 || size > 8 {
		return nil
	}

	sizeField := byte(size-1) << 4 // SSS shifted to bits 6:4

	if pkg.Type <= 0x0E {
		header := byte(0x80) | sizeField | byte(pkg.Type)
		result := make([]byte, 1+size)
		result[0] = header
		copy(result[1:], pkg.Payload)
		return result
	}

	// Extended type: TTTT = 0x0F, next byte = type - 15
	header := byte(0x80) | sizeField | 0x0F
	extType := byte(pkg.Type - 15)
	result := make([]byte, 2+size)
	result[0] = header
	result[1] = extType
	copy(result[2:], pkg.Payload)
	return result
}

// DecodeMiniPkg decodes one MiniPkg sub-packet from the given data.
// Returns the decoded packet and the number of bytes consumed, or an error.
func DecodeMiniPkg(data []byte) (MiniPkg, int, error) {
	if len(data) < 1 {
		return MiniPkg{}, 0, fmt.Errorf("empty data")
	}

	header := data[0]
	sizeField := int((header >> 4) & 0x07) // bits 6:4
	payloadSize := sizeField + 1
	typeField := int(header & 0x0F)

	consumed := 1 // header byte
	pkgType := typeField

	if typeField == 0x0F {
		// Extended type
		if len(data) < 2 {
			return MiniPkg{}, 0, fmt.Errorf("extended type but no type byte")
		}
		pkgType = int(data[1]) + 15
		consumed = 2 // header + extended type byte
	}

	if len(data) < consumed+payloadSize {
		return MiniPkg{}, 0, fmt.Errorf("not enough data: need %d, have %d", consumed+payloadSize, len(data))
	}

	payload := make([]byte, payloadSize)
	copy(payload, data[consumed:consumed+payloadSize])

	return MiniPkg{Type: pkgType, Payload: payload}, consumed + payloadSize, nil
}

// DecodeMiniPkgs decodes all MiniPkg sub-packets from data.
func DecodeMiniPkgs(data []byte) ([]MiniPkg, error) {
	var pkgs []MiniPkg
	offset := 0
	for offset < len(data) {
		pkg, n, err := DecodeMiniPkg(data[offset:])
		if err != nil {
			return pkgs, fmt.Errorf("at offset %d: %w", offset, err)
		}
		pkgs = append(pkgs, pkg)
		offset += n
	}
	return pkgs, nil
}

// MiniPkgSourceTag creates a Source sub-packet.
func MiniPkgSourceTag(source byte) MiniPkg {
	return MiniPkg{Type: MiniPkgSource, Payload: []byte{source}}
}

// MiniPkgWhiteBalanceTag creates a WhiteBalance sub-packet with temperature in Kelvin (big-endian).
func MiniPkgWhiteBalanceTag(kelvin uint16) MiniPkg {
	return MiniPkg{Type: MiniPkgWhiteBalance, Payload: []byte{byte(kelvin >> 8), byte(kelvin & 0xFF)}}
}

// MiniPkgWindowControlSet creates a WindowControl sub-packet for setting position.
func MiniPkgWindowControlSet(position uint16) MiniPkg {
	return MiniPkg{Type: MiniPkgWindowCtrl, Payload: []byte{0x01, byte(position >> 8), byte(position & 0xFF)}}
}

// MiniPkgWindowControlStop creates a WindowControl sub-packet for stopping.
func MiniPkgWindowControlStop() MiniPkg {
	return MiniPkg{Type: MiniPkgWindowCtrl, Payload: []byte{0x00}}
}

// MiniPkgTiltTag creates a Tilt sub-packet with the given angle byte.
func MiniPkgTiltTag(angle byte) MiniPkg {
	return MiniPkg{Type: MiniPkgTilt, Payload: []byte{angle}}
}
