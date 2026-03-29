# justplejd-cli

A Go CLI that controls [Plejd](https://www.plejd.com/) smart home devices over Bluetooth Low Energy (BLE). Authenticates via the Plejd cloud API to retrieve encryption keys and site configuration, then communicates directly with Plejd mesh devices over BLE.

## Installation

```bash
go install github.com/gloriaoracle/justplejd-cli/cmd/justplejd@latest
```

Or build from source:

```bash
git clone <repo>
cd justplejd-cli
go build -o justplejd ./cmd/justplejd/
```

Requires a Bluetooth adapter. On macOS, the built-in Bluetooth works out of the box.

## Quick Start

```bash
# 1. Login to Plejd cloud (retrieves crypto keys and device config)
justplejd login --email you@example.com --password secret

# 2. List your devices
justplejd devices

# 3. Control devices
justplejd on "Ceiling Light"
justplejd off "Ceiling Light"
justplejd dim "Ceiling Light" 75%
justplejd scene "Movie"
```

## Commands

| Command | Description |
|---------|-------------|
| `login` | Authenticate with Plejd cloud API |
| `devices` | List all devices with room, address, and traits |
| `rooms` | List rooms and their devices |
| `scenes` | List scenes with indices |
| `on DEVICE` | Turn on (by name or address) |
| `off DEVICE` | Turn off |
| `dim DEVICE LEVEL` | Dim (0-255 or 0-100%) |
| `scene SCENE` | Activate scene (by name or index) |
| `listen` | Listen for mesh events in real-time |
| `status` | Show connection status and site info |
| `time [--set]` | Get or set mesh time |

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Verbose output (connection details, scan info) |
| `--json` | JSON output format |
| `--config PATH` | Config file path (default `~/.config/justplejd/config.json`) |
| `--timeout DURATION` | BLE scan timeout (default `3s`) |

## Usage Examples

### Login

```bash
# Single site account
justplejd login --email you@example.com --password secret

# Multiple sites — specify which one
justplejd login --email you@example.com --password secret --site-id <uuid>
```

### Control Devices

```bash
# By name (case-insensitive)
justplejd on "kitchen light"
justplejd off "Kitchen Light"

# By address (from 'devices' output)
justplejd on 39
justplejd off 39

# Dim with percentage
justplejd dim "Ceiling Light" 50%

# Dim with raw value (0-255)
justplejd dim "Ceiling Light" 128
```

### Scenes

```bash
# By name
justplejd scene "Movie"

# By index
justplejd scene 1
```

### Listen for Events

```bash
# Plain text output
justplejd listen
# [STATE] addr=39 ON
# [DIM] addr=39 state=1 dim=182
# [SCENE] Scene 1 activated

# JSON output (for scripting)
justplejd listen --json
# {"Type":"change_state","Address":39,"State":1,...}
```

### JSON Output

```bash
# All list commands support --json
justplejd devices --json
justplejd rooms --json
justplejd scenes --json

# Control commands report action taken
justplejd on "Ceiling Light" --json
# {"device":"Ceiling Light","action":"on","address":39}
```

## Architecture

```
cmd/justplejd/     CLI entry point (cobra), config management
pkg/api/           Plejd cloud HTTP API client
pkg/ble/           BLE communication layer (interface + real adapter)
pkg/crypto/        AES-ECB encryption, auth challenge-response
pkg/protocol/      Plejd mesh protocol commands and packet parser
internal/testutil/ Mock BLE adapter for testing
```

The BLE layer uses an interface (`ble.Adapter`, `ble.Device`) for testability. Tests run against a mock adapter without requiring Bluetooth hardware.

## Protocol Research

### BLE Service

Plejd devices advertise as "P mesh" with manufacturer data (company ID `0x0377` / 887). The BLE service UUID is `31ba0001-6085-4726-be45-040c957391b5`.

### Encryption

All data is encrypted using AES-ECB as a keystream generator:
1. Build a 16-byte buffer from the reversed MAC address (repeated)
2. AES-ECB encrypt it with the site's crypto key to get a keystream
3. XOR data with the keystream (wrapping every 16 bytes)

### Authentication

Challenge-response using SHA256:
1. Write `0x00` to trigger challenge generation
2. Read 16-byte challenge
3. XOR the crypto key and challenge as big-endian integers
4. SHA256 hash the result, then XOR the two halves for a 16-byte response

### Device Traits (Bitmask)

| Bit | Value | Trait |
|-----|-------|-------|
| 0 | 0x01 | GROUP |
| 1 | 0x02 | DIM |
| 2 | 0x04 | TEMP (color temperature) |
| 3 | 0x08 | POWER (has power output) |
| 4 | 0x10 | COVER |
| 6 | 0x40 | TILT |

### Known Hardware Types

| ID | Model | Type |
|----|-------|------|
| 1 | DIM-01 | Dimmer |
| 2 | DIM-02 | Dimmer |
| 3 | CTR-01 | Controller |
| 4 | GWY-01 | Gateway |
| 5 | LED-10 | LED dimmer |
| 6 | WPH-01 | Wall panel |
| 7 | REL-01 | Relay |
| 8 | SPR-01 | Smart plug |
| 10 | WRT-01 | Rotary |
| 11 | DIM-01-2P | 2-pole dimmer |
| 14 | DIM-01-LC | Dimmer LC |
| 17 | REL-01-2P | 2-pole relay |
| 18 | REL-02 | Relay |
| 36 | LED-75 | LED dimmer |
| 70 | WMS-01 | Motion sensor |
| 103 | OUT-01 | Outdoor (dim+colortemp) |
| 167 | DWN-01 | Downlight (dim+colortemp) |
| 199 | DWN-02 | Downlight (dim+colortemp) |

### Future Extension Ideas

These protocol features have been observed in reference implementations but are not yet implemented:

- **Thermostat control**: Set point, operating mode (Floor/Room/PWM), PWM duty cycle (commands `0x045C`, `0x045F`, `0x0461`, `0x047E`)
- **Cover tilt angle**: Fine-grained tilt control beyond position (MiniPkg type `0x18`)
- **Firmware version query**: Read firmware info from `plejdDevices` data
- **Battery status**: Low-power battery info from motion sensors (MiniPkg type `0x16`)
- **Mesh topology mapping**: Use device addressing and group info to map the mesh network
- **Raw packet capture**: Log all decrypted packets with hex dumps for protocol analysis
- **Group addressing**: Send commands to device groups rather than individual addresses
- **Motion sensor lux levels**: Ambient light reporting from WMS-01 sensors (MiniPkg type `0x06`)
- **Device configuration**: Read/write device settings via the mesh protocol
- **OTA updates**: Firmware update mechanism investigation

## Testing

```bash
# Run all tests
go test ./...

# Verbose
go test ./... -v

# Specific package
go test ./pkg/crypto/ -v
```

Tests cover:
- **Crypto**: encrypt/decrypt roundtrip, auth challenge-response with known vectors, MAC extraction
- **Protocol**: all command encodings against expected hex, packet parsing for all event types
- **API**: HTTP client against httptest mock server (login, sites, site details)
- **BLE**: scan/filter, authenticate, send commands, ping, disconnect against mock adapter
