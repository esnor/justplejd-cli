package crypto

import (
	"encoding/hex"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	// EncryptDecrypt is self-inverse: applying it twice returns the original data
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	macAddr := "AA:BB:CC:DD:EE:FF"
	original := []byte{0x27, 0x01, 0x10, 0x00, 0x97, 0x01}

	encrypted := EncryptDecrypt(cryptoKey, macAddr, original)
	if encrypted == nil {
		t.Fatal("EncryptDecrypt returned nil")
	}
	if len(encrypted) != len(original) {
		t.Fatalf("encrypted length %d != original length %d", len(encrypted), len(original))
	}

	// Encrypting again should give back original
	decrypted := EncryptDecrypt(cryptoKey, macAddr, encrypted)
	if len(decrypted) != len(original) {
		t.Fatalf("decrypted length %d != original length %d", len(decrypted), len(original))
	}
	for i := range original {
		if decrypted[i] != original[i] {
			t.Errorf("byte %d: got %02x, want %02x", i, decrypted[i], original[i])
		}
	}
}

func TestEncryptDecryptLongerThan16Bytes(t *testing.T) {
	// Data longer than 16 bytes should wrap the keystream
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	macAddr := "AA:BB:CC:DD:EE:FF"
	original := make([]byte, 20)
	for i := range original {
		original[i] = byte(i)
	}

	encrypted := EncryptDecrypt(cryptoKey, macAddr, original)
	if encrypted == nil {
		t.Fatal("EncryptDecrypt returned nil")
	}
	if len(encrypted) != 20 {
		t.Fatalf("encrypted length %d != 20", len(encrypted))
	}

	decrypted := EncryptDecrypt(cryptoKey, macAddr, encrypted)
	for i := range original {
		if decrypted[i] != original[i] {
			t.Errorf("byte %d: got %02x, want %02x", i, decrypted[i], original[i])
		}
	}
}

func TestEncryptDecryptDifferentKeys(t *testing.T) {
	macAddr := "AA:BB:CC:DD:EE:FF"
	data := []byte{0x01, 0x02, 0x03, 0x04}

	enc1 := EncryptDecrypt("0123456789ABCDEF0123456789ABCDEF", macAddr, data)
	enc2 := EncryptDecrypt("FEDCBA9876543210FEDCBA9876543210", macAddr, data)

	if enc1 == nil || enc2 == nil {
		t.Fatal("EncryptDecrypt returned nil")
	}

	same := true
	for i := range enc1 {
		if enc1[i] != enc2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different keys produced identical ciphertext")
	}
}

func TestEncryptDecryptDifferentAddresses(t *testing.T) {
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	data := []byte{0x01, 0x02, 0x03, 0x04}

	enc1 := EncryptDecrypt(cryptoKey, "AA:BB:CC:DD:EE:FF", data)
	enc2 := EncryptDecrypt(cryptoKey, "11:22:33:44:55:66", data)

	if enc1 == nil || enc2 == nil {
		t.Fatal("EncryptDecrypt returned nil")
	}

	same := true
	for i := range enc1 {
		if enc1[i] != enc2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different MAC addresses produced identical ciphertext")
	}
}

func TestEncryptDecryptEmptyData(t *testing.T) {
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	macAddr := "AA:BB:CC:DD:EE:FF"

	result := EncryptDecrypt(cryptoKey, macAddr, []byte{})
	if result == nil {
		t.Fatal("EncryptDecrypt returned nil for empty data")
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d bytes", len(result))
	}
}

func TestEncryptDecryptNoColonMAC(t *testing.T) {
	// MAC address without colons should work the same
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	data := []byte{0x01, 0x02, 0x03, 0x04}

	enc1 := EncryptDecrypt(cryptoKey, "AA:BB:CC:DD:EE:FF", data)
	enc2 := EncryptDecrypt(cryptoKey, "AABBCCDDEEFF", data)

	if enc1 == nil || enc2 == nil {
		t.Fatal("EncryptDecrypt returned nil")
	}

	for i := range enc1 {
		if enc1[i] != enc2[i] {
			t.Errorf("byte %d: colon MAC gave %02x, no-colon gave %02x", i, enc1[i], enc2[i])
		}
	}
}

func TestAuthResponseLength(t *testing.T) {
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	challenge := make([]byte, 16)

	response := AuthResponse(cryptoKey, challenge)
	if response == nil {
		t.Fatal("AuthResponse returned nil")
	}
	if len(response) != 16 {
		t.Fatalf("expected 16-byte response, got %d", len(response))
	}
}

func TestAuthResponseDeterministic(t *testing.T) {
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	challenge := []byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA, 0x99, 0x88,
		0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}

	resp1 := AuthResponse(cryptoKey, challenge)
	resp2 := AuthResponse(cryptoKey, challenge)

	if resp1 == nil || resp2 == nil {
		t.Fatal("AuthResponse returned nil")
	}

	for i := range resp1 {
		if resp1[i] != resp2[i] {
			t.Errorf("byte %d: first call %02x != second call %02x", i, resp1[i], resp2[i])
		}
	}
}

func TestAuthResponseDifferentChallenges(t *testing.T) {
	cryptoKey := "0123456789ABCDEF0123456789ABCDEF"
	challenge1 := make([]byte, 16)
	challenge2 := make([]byte, 16)
	challenge2[0] = 0x01

	resp1 := AuthResponse(cryptoKey, challenge1)
	resp2 := AuthResponse(cryptoKey, challenge2)

	if resp1 == nil || resp2 == nil {
		t.Fatal("AuthResponse returned nil")
	}

	same := true
	for i := range resp1 {
		if resp1[i] != resp2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different challenges produced identical responses")
	}
}

func TestAuthResponseKnownVector(t *testing.T) {
	// Test vector: key and challenge all zeros
	// k = 0, c = 0, k^c = 0
	// intermediate = SHA256(16 zero bytes)
	// response = intermediate[:16] XOR intermediate[16:]
	cryptoKey := "00000000000000000000000000000000"
	challenge := make([]byte, 16)

	response := AuthResponse(cryptoKey, challenge)
	if response == nil {
		t.Fatal("AuthResponse returned nil")
	}

	// SHA256 of 16 zero bytes = 374708fff7719dd5979ec875d56cd2286f6d3cf7ec317a3b25632aab28ec37bb
	// part1 = 374708fff7719dd5979ec875d56cd228
	// part2 = 6f6d3cf7ec317a3b25632aab28ec37bb
	// XOR   = 582a34087040e7eeb2fde2defD80e593
	expected := "582a34081b40e7eeb2fde2defd80e593"
	got := hex.EncodeToString(response)
	if got != expected {
		t.Errorf("auth response:\n  got  %s\n  want %s", got, expected)
	}
}

func TestExtractMACAddress(t *testing.T) {
	// manufacturer data bytes [4:10] reversed = MAC
	mdata := []byte{0x00, 0x00, 0x00, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	mac := ExtractMACAddress(mdata)
	if mac != "66:55:44:33:22:11" {
		t.Errorf("got %q, want %q", mac, "66:55:44:33:22:11")
	}
}

func TestExtractMACAddressTooShort(t *testing.T) {
	mdata := []byte{0x00, 0x00, 0x00}
	mac := ExtractMACAddress(mdata)
	if mac != "" {
		t.Errorf("expected empty string for short data, got %q", mac)
	}
}

func TestExtractMACAddressNil(t *testing.T) {
	mac := ExtractMACAddress(nil)
	if mac != "" {
		t.Errorf("expected empty string for nil data, got %q", mac)
	}
}
