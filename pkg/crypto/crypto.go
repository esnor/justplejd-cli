// Package crypto implements Plejd BLE encryption/decryption and authentication.
package crypto

import (
	"crypto/aes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// EncryptDecrypt performs the Plejd encrypt/decrypt operation.
// It is a self-inverse XOR stream cipher using AES-ECB as keystream generator.
// cryptoKey is a hex string (32 chars = 16 bytes).
// macAddr is a MAC address string (with or without colons/dashes).
func EncryptDecrypt(cryptoKey string, macAddr string, data []byte) []byte {
	key, err := hex.DecodeString(strings.ReplaceAll(strings.ReplaceAll(cryptoKey, "-", ""), ":", ""))
	if err != nil {
		return nil
	}

	addr, err := hex.DecodeString(strings.ReplaceAll(strings.ReplaceAll(macAddr, "-", ""), ":", ""))
	if err != nil {
		return nil
	}

	// Reverse the address bytes
	for i, j := 0, len(addr)-1; i < j; i, j = i+1, j-1 {
		addr[i], addr[j] = addr[j], addr[i]
	}

	// Build 16-byte buffer: addr + addr + addr[:4]
	var buf [16]byte
	copy(buf[0:6], addr)
	copy(buf[6:12], addr)
	copy(buf[12:16], addr[:4])

	// AES-ECB encrypt the buffer to generate keystream
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}

	var keystream [16]byte
	block.Encrypt(keystream[:], buf[:])

	// XOR data with keystream (wrapping at 16 bytes)
	output := make([]byte, len(data))
	for i, d := range data {
		output[i] = d ^ keystream[i%16]
	}
	return output
}

// AuthResponse computes the BLE auth challenge-response.
// cryptoKey is a hex string, challenge is 16 raw bytes from the device.
// Returns a 16-byte response to write back.
func AuthResponse(cryptoKey string, challenge []byte) []byte {
	key, err := hex.DecodeString(strings.ReplaceAll(strings.ReplaceAll(cryptoKey, "-", ""), ":", ""))
	if err != nil {
		return nil
	}

	// Convert key and challenge to big integers, XOR them
	k := new(big.Int).SetBytes(key)
	c := new(big.Int).SetBytes(challenge)
	xored := new(big.Int).Xor(k, c)

	// Encode as 16 big-endian bytes
	xoredBytes := make([]byte, 16)
	b := xored.Bytes()
	// Right-align in the 16-byte buffer
	copy(xoredBytes[16-len(b):], b)

	// SHA256 hash
	intermediate := sha256.Sum256(xoredBytes)

	// XOR first 16 bytes with last 16 bytes
	response := make([]byte, 16)
	for i := 0; i < 16; i++ {
		response[i] = intermediate[i] ^ intermediate[i+16]
	}
	return response
}

// ExtractMACAddress extracts a MAC address from BLE manufacturer data.
// Manufacturer data bytes [4:10] reversed = MAC address.
// Returns colon-separated uppercase hex string like "AA:BB:CC:DD:EE:FF".
func ExtractMACAddress(manufacturerData []byte) string {
	if len(manufacturerData) < 10 {
		return ""
	}

	mac := manufacturerData[4:10]
	// Reverse to get actual MAC order
	parts := make([]string, 6)
	for i := 0; i < 6; i++ {
		parts[i] = fmt.Sprintf("%02X", mac[5-i])
	}
	return strings.Join(parts, ":")
}
