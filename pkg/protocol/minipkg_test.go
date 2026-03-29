package protocol

import (
	"encoding/hex"
	"testing"
)

func TestEncodeMiniPkgSimpleType(t *testing.T) {
	// Source=APP (type 0x03, 1 byte payload [0x08])
	// Header: 0x80 | (0 << 4) | 0x03 = 0x83
	pkg := MiniPkg{Type: MiniPkgSource, Payload: []byte{SourceApp}}
	got := EncodeMiniPkg(pkg)
	want := "8308"
	if hex.EncodeToString(got) != want {
		t.Errorf("Source(APP): got %s, want %s", hex.EncodeToString(got), want)
	}
}

func TestEncodeMiniPkgWhiteBalance(t *testing.T) {
	// WhiteBalance type 0x01, 2 bytes payload (3000K = 0x0BB8)
	// Header: 0x80 | (1 << 4) | 0x01 = 0x91
	pkg := MiniPkgWhiteBalanceTag(3000)
	got := EncodeMiniPkg(pkg)
	want := "910bb8"
	if hex.EncodeToString(got) != want {
		t.Errorf("WhiteBalance(3000): got %s, want %s", hex.EncodeToString(got), want)
	}
}

func TestEncodeMiniPkgWindowControl(t *testing.T) {
	// WindowControl type 0x07, 3 bytes [0x01, 0x00, 0xFF]
	// Header: 0x80 | (2 << 4) | 0x07 = 0xA7
	pkg := MiniPkgWindowControlSet(0x00FF)
	got := EncodeMiniPkg(pkg)
	want := "a70100ff"
	if hex.EncodeToString(got) != want {
		t.Errorf("WindowControl(255): got %s, want %s", hex.EncodeToString(got), want)
	}
}

func TestEncodeMiniPkgWindowControlStop(t *testing.T) {
	// WindowControl stop: type 0x07, 1 byte [0x00]
	// Header: 0x80 | (0 << 4) | 0x07 = 0x87
	pkg := MiniPkgWindowControlStop()
	got := EncodeMiniPkg(pkg)
	want := "8700"
	if hex.EncodeToString(got) != want {
		t.Errorf("WindowControlStop: got %s, want %s", hex.EncodeToString(got), want)
	}
}

func TestEncodeMiniPkgExtendedType(t *testing.T) {
	// Channel type 0x10 -> extended: TTTT=0x0F, ext byte = 0x10 - 15 = 1
	// Header: 0x80 | (0 << 4) | 0x0F = 0x8F
	pkg := MiniPkg{Type: MiniPkgChannel, Payload: []byte{0x01}}
	got := EncodeMiniPkg(pkg)
	want := "8f0101"
	if hex.EncodeToString(got) != want {
		t.Errorf("Channel(1): got %s, want %s", hex.EncodeToString(got), want)
	}
}

func TestEncodeMiniPkgBatteryInfo(t *testing.T) {
	// BatteryInfo type 0x16, 2 bytes
	// Extended: type > 0x0E -> TTTT=0x0F, ext = 0x16-15 = 7
	// Header: 0x80 | (1 << 4) | 0x0F = 0x9F
	pkg := MiniPkg{Type: MiniPkgBatteryInfo, Payload: []byte{0x0C, 0x80}}
	got := EncodeMiniPkg(pkg)
	want := "9f070c80"
	if hex.EncodeToString(got) != want {
		t.Errorf("BatteryInfo: got %s, want %s", hex.EncodeToString(got), want)
	}
}

func TestEncodeMiniPkgTilt(t *testing.T) {
	// Tilt type 0x18, 1 byte
	// Extended: TTTT=0x0F, ext = 0x18-15 = 9
	// Header: 0x80 | (0 << 4) | 0x0F = 0x8F
	pkg := MiniPkgTiltTag(45)
	got := EncodeMiniPkg(pkg)
	want := "8f092d"
	if hex.EncodeToString(got) != want {
		t.Errorf("Tilt(45): got %s, want %s", hex.EncodeToString(got), want)
	}
}

func TestEncodeMiniPkgInvalidSize(t *testing.T) {
	// Empty payload should return nil
	pkg := MiniPkg{Type: MiniPkgSource, Payload: []byte{}}
	got := EncodeMiniPkg(pkg)
	if got != nil {
		t.Errorf("empty payload: expected nil, got %x", got)
	}

	// Payload > 8 bytes should return nil
	pkg = MiniPkg{Type: MiniPkgSource, Payload: make([]byte, 9)}
	got = EncodeMiniPkg(pkg)
	if got != nil {
		t.Errorf("oversized payload: expected nil, got %x", got)
	}
}

func TestDecodeMiniPkgSimple(t *testing.T) {
	// Source=APP: 83 08
	data := []byte{0x83, 0x08}
	pkg, n, err := DecodeMiniPkg(data)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if n != 2 {
		t.Errorf("consumed: got %d, want 2", n)
	}
	if pkg.Type != MiniPkgSource {
		t.Errorf("type: got %d, want %d", pkg.Type, MiniPkgSource)
	}
	if len(pkg.Payload) != 1 || pkg.Payload[0] != SourceApp {
		t.Errorf("payload: got %x, want [08]", pkg.Payload)
	}
}

func TestDecodeMiniPkgExtended(t *testing.T) {
	// Tilt: 8F 09 2D (type=0x0F ext, ext=9 -> 15+9=24=0x18, payload=[0x2D])
	data := []byte{0x8F, 0x09, 0x2D}
	pkg, n, err := DecodeMiniPkg(data)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if n != 3 {
		t.Errorf("consumed: got %d, want 3", n)
	}
	if pkg.Type != MiniPkgTilt {
		t.Errorf("type: got %d, want %d", pkg.Type, MiniPkgTilt)
	}
	if len(pkg.Payload) != 1 || pkg.Payload[0] != 0x2D {
		t.Errorf("payload: got %x, want [2d]", pkg.Payload)
	}
}

func TestMiniPkgRoundTrip(t *testing.T) {
	cases := []MiniPkg{
		{Type: MiniPkgSource, Payload: []byte{SourceApp}},
		{Type: MiniPkgSource, Payload: []byte{SourceMotion}},
		{Type: MiniPkgWhiteBalance, Payload: []byte{0x0B, 0xB8}},
		{Type: MiniPkgLux, Payload: []byte{0x01}},
		{Type: MiniPkgLux, Payload: []byte{0x02}},
		{Type: MiniPkgWindowCtrl, Payload: []byte{0x01, 0x00, 0x64}},
		{Type: MiniPkgWindowCtrl, Payload: []byte{0x00}},
		{Type: MiniPkgChannel, Payload: []byte{0x01}},
		{Type: MiniPkgBatteryInfo, Payload: []byte{0x0C, 0x80}},
		{Type: MiniPkgTilt, Payload: []byte{0x2D}},
	}

	for _, tc := range cases {
		encoded := EncodeMiniPkg(tc)
		if encoded == nil {
			t.Errorf("encode type=%d: got nil", tc.Type)
			continue
		}
		decoded, n, err := DecodeMiniPkg(encoded)
		if err != nil {
			t.Errorf("decode type=%d: %v", tc.Type, err)
			continue
		}
		if n != len(encoded) {
			t.Errorf("type=%d consumed %d, want %d", tc.Type, n, len(encoded))
		}
		if decoded.Type != tc.Type {
			t.Errorf("type: got %d, want %d", decoded.Type, tc.Type)
		}
		if len(decoded.Payload) != len(tc.Payload) {
			t.Errorf("type=%d payload length: got %d, want %d", tc.Type, len(decoded.Payload), len(tc.Payload))
			continue
		}
		for i := range tc.Payload {
			if decoded.Payload[i] != tc.Payload[i] {
				t.Errorf("type=%d payload[%d]: got %02x, want %02x", tc.Type, i, decoded.Payload[i], tc.Payload[i])
			}
		}
	}
}

func TestDecodeMiniPkgs(t *testing.T) {
	// Encode Source(APP) + WhiteBalance(3000) and decode both
	src := EncodeMiniPkg(MiniPkgSourceTag(SourceApp))
	wb := EncodeMiniPkg(MiniPkgWhiteBalanceTag(3000))
	combined := append(src, wb...)

	pkgs, err := DecodeMiniPkgs(combined)
	if err != nil {
		t.Fatalf("DecodeMiniPkgs error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packets, got %d", len(pkgs))
	}
	if pkgs[0].Type != MiniPkgSource {
		t.Errorf("first packet type: got %d, want %d", pkgs[0].Type, MiniPkgSource)
	}
	if pkgs[1].Type != MiniPkgWhiteBalance {
		t.Errorf("second packet type: got %d, want %d", pkgs[1].Type, MiniPkgWhiteBalance)
	}
}

func TestDecodeMiniPkgErrors(t *testing.T) {
	// Empty data
	_, _, err := DecodeMiniPkg([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}

	// Extended type but no type byte
	_, _, err = DecodeMiniPkg([]byte{0x8F})
	if err == nil {
		t.Error("expected error for truncated extended type")
	}

	// Not enough data for payload
	_, _, err = DecodeMiniPkg([]byte{0x91}) // size=2 but only 1 byte total
	if err == nil {
		t.Error("expected error for insufficient payload data")
	}
}
