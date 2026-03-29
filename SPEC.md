# justplejd-cli — Go CLI for Plejd Smart Home Devices

## Overview
A self-contained Go CLI that communicates with Plejd smart home devices over BLE (Bluetooth Low Energy). It authenticates via the Plejd cloud API to retrieve crypto keys and site configuration, then connects directly to Plejd mesh devices over BLE.

## Architecture

### Packages
- `cmd/justplejd/` — CLI entry point (cobra)
- `pkg/api/` — Plejd cloud HTTP API client
- `pkg/ble/` — BLE communication layer (scanning, connecting, auth, read/write)
- `pkg/crypto/` — Encryption/decryption, auth challenge-response
- `pkg/protocol/` — Plejd mesh protocol: commands, packet parsing, payload encoding
- `internal/testutil/` — Mock BLE interface for testing

### Config / Auth
- Store auth/config in `~/.config/justplejd/config.json`
- Fields: email, site_id, crypto_key, cached site data (devices, rooms, scenes)
- On first use: `justplejd login --email foo@bar.com --password secret`
  - Calls Plejd cloud API, retrieves sites, stores crypto_key + site data

## Plejd Cloud API
- Base URL: `https://cloud.plejd.com`
- App ID header: `X-Parse-Application-Id: zHtVqXt8k4yFyk2QGmgp48D9xZr2G94xWYnF4dak`
- Login: `POST /parse/login` with `{"username": email, "password": password}` → sessionToken
- List sites: `POST /parse/functions/getSiteList` with session token → site list
- Get site details: `POST /parse/functions/getSiteById` with `siteId` → full site config

### Site Data Structure
```json
{
  "site": {"siteId": "uuid", "title": "Home"},
  "plejdMesh": {"cryptoKey": "hex-string"},
  "rooms": [{"roomId": "uuid", "title": "Kitchen"}],
  "devices": [{"deviceId": "hex", "title": "Ceiling Light", "roomId": "uuid", "traits": 10}],
  "deviceAddress": {"deviceId": address_int},
  "scenes": [{"sceneId": "uuid", "title": "Movie"}],
  "sceneIndex": {"sceneId": address_int}
}
```

### Device Traits (bitmask)
- 0x01 = GROUP
- 0x02 = DIM
- 0x04 = TEMP (color temperature)
- 0x08 = POWER (has power output — needed for gateway selection)
- 0x10 = COVER
- 0x40 = TILT

## BLE Protocol

### Service & Characteristics
- Service UUID: `31ba0001-6085-4726-be45-040c957391b5`
- Data Write: `31ba0004-6085-4726-be45-040c957391b5`
- Data Notify (last data): `31ba0005-6085-4726-be45-040c957391b5`
- Auth: `31ba0009-6085-4726-be45-040c957391b5`
- Ping: `31ba000a-6085-4726-be45-040c957391b5`

### Device Discovery
1. BLE scan for devices advertising name starting with "P mesh"
2. Extract MAC address from manufacturer data (company ID 887/0x0377):
   - Manufacturer data bytes [4:10] reversed = MAC address
3. Match MAC against known device IDs from site config (devices with POWER trait)
4. Pick device with strongest RSSI as gateway

### Authentication
1. Write `0x00` to AUTH characteristic
2. Wait ~1 second
3. Read AUTH characteristic → 16-byte challenge
4. Compute response:
   - key = hex_to_bytes(crypto_key)
   - k = big_endian_int(key), c = big_endian_int(challenge)
   - intermediate = SHA256((k XOR c) as 16 big-endian bytes)
   - response = intermediate[:16] XOR intermediate[16:]
5. Write response to AUTH characteristic
6. Verify with ping: write random byte to PING, read back, expect (byte+1) & 0xFF

### Encryption/Decryption
All data sent/received on the Data characteristic is encrypted:
1. key = hex_to_bytes(crypto_key)
2. addr = hex_to_bytes(mac_address) reversed
3. buf = addr + addr + addr[:4] (16 bytes)
4. keystream = AES-ECB-encrypt(key, buf)
5. output[i] = data[i] XOR keystream[i % 16]

### Command Format
Commands are hex payloads that get encrypted before sending:
```
AA VVTT CCCC [payload]

AA = device address (1 byte)
VV = version (always 01)
TT = command type (10 = write-no-respond, 00 = write, 01 = ack, 02 = read)
CCCC = command code (2 bytes big-endian)
payload = command-specific data
```

### Commands
| Command | Code | Format | Description |
|---------|------|--------|-------------|
| Turn Off | 0x0097 | `AA 0110 0097 00` | Turn device off |
| Turn On | 0x0097 | `AA 0110 0097 01` | Turn device on |
| Dim | 0x0098 | `AA 0110 0098 01 DDDD` | Set dim level (0-255, sent twice) |
| Color Temp | 0x0420 | `AA 0110 0420 030111 TTTT` | Set color temperature |
| Cover | 0x0420 | `AA 0110 0420 030827 01 PPPP` | Set cover position |
| Cover Stop | 0x0420 | `AA 0110 0420 030807 00` | Stop cover |
| Scene | 0x0021 | `02 0110 0021 II` | Activate scene (II = scene index) |
| Set Time | 0x001B | `00 0110 001B TTTTTTTTTT` | Set mesh time (5 bytes LE timestamp) |
| Request Time | 0x001B | `AA 0102 001B` | Request time from device |

### Incoming Packet Parsing
Notifications on the Data Notify characteristic (after decryption):
| Pattern | Event |
|---------|-------|
| `00 0110 0021 SS` | Scene activated (SS = scene index) |
| `00 0110 0016 AA BB [xx]` | Button press (AA=addr, BB=button, xx=0 release) |
| `AA 0110 00C8 SS DD DD [...]` or `AA 0110 0098 SS DD DD [...]` | Dim state change |
| `AA 0110 0097 SS` | On/Off state change |
| `AA 0110 0420 01 0111 TTTT` | Color temperature change |
| `AA 0110 0420 03 xx ... LL LL` | Motion event |

## CLI Interface

### Commands
```
justplejd login --email EMAIL --password PASSWORD [--site-id SITE_ID]
justplejd devices                    # List all devices with room, address, traits
justplejd rooms                      # List rooms and their devices
justplejd scenes                     # List scenes with indices
justplejd on DEVICE                  # Turn on (by name or address)
justplejd off DEVICE                 # Turn off
justplejd dim DEVICE LEVEL           # Dim (0-255 or 0-100%)
justplejd scene SCENE                # Activate scene (by name or index)
justplejd status                     # Show connection status
justplejd listen                     # Listen for mesh events (real-time)
justplejd time [--set]               # Get/set mesh time
```

### Flags
```
--verbose, -v          Verbose output
--json                 JSON output format
--config PATH          Config file path (default ~/.config/justplejd/config.json)
--timeout DURATION     BLE scan timeout (default 3s)
```

## Testing Strategy
Create a mock BLE interface in `internal/testutil/` that:
1. Simulates BLE scanning (returns fake Plejd devices with manufacturer data)
2. Simulates GATT read/write on characteristics
3. Records all writes for assertion
4. Can inject notifications (incoming data)

Test coverage:
- `pkg/crypto/` — encrypt/decrypt roundtrip, auth challenge-response (use known test vectors from Python)
- `pkg/protocol/` — command encoding, packet parsing
- `pkg/api/` — HTTP API client with httptest mock server
- `pkg/ble/` — BLE operations against mock interface
- Integration — full flow from login to command execution

## Hardware Types (from protocol research)
```
0: unknown
1: DIM-01 (dimmer)
2: DIM-02 (dimmer)
3: CTR-01 (controller)
4: GWY-01 (gateway)
5: LED-10 (LED dimmer)
6: WPH-01 (wall panel)
7: REL-01 (relay)
8: SPR-01 (smart plug)
10: WRT-01 (rotary)
11: DIM-01-2P (2-pole dimmer)
14: DIM-01-LC (dimmer LC)
15: DIM-02-LC (dimmer LC)
17: REL-01-2P (2-pole relay)
18: REL-02 (relay)
36: LED-75 (LED dimmer)
70: WMS-01 (motion sensor)
103: OUT-01 (outdoor, dim+colortemp)
167: DWN-01 (downlight, dim+colortemp)
199: DWN-02 (downlight, dim+colortemp)
```

## Dependencies
- `github.com/spf13/cobra` — CLI framework
- `tinygo.org/x/bluetooth` or `github.com/muka/go-bluetooth` — BLE library (evaluate which works best on macOS)
- Standard library for crypto (crypto/aes, crypto/sha256)

## Future Protocol Extensions (Phase 2)
After initial port is verified working:
- Raw packet capture mode for protocol analysis
- Unknown command logging with hex dumps
- Firmware version query
- Battery level reporting (for motion sensors)
- Thermostat control (set point, operating mode, PWM)
- Cover tilt angle control
- Group addressing
- Mesh network topology mapping
- Device configuration read/write
- OTA update investigation
