# Plejd BLE Mesh Protocol — Reverse Engineering Notes

Compiled from: pyplejd (thomasloven), justplejd (Jesper), hassio-plejd (icanos),
ha-plejd (klali), and community issue tracker discussions.

---

## 1. BLE Service & Characteristics

Service UUID: `31ba0001-6085-4726-be45-040c957391b5`

| Name | UUID suffix | Purpose |
|------|-------------|---------|
| LIGHTLEVEL | `31ba0003-...` | Poll/subscribe for device state snapshots (10 bytes per device) |
| DATA | `31ba0004-...` | Write encrypted commands to the mesh |
| LASTDATA | `31ba0005-...` | Subscribe for encrypted event notifications from the mesh |
| AUTH | `31ba0009-...` | Authentication handshake |
| PING | `31ba000a-...` | Keepalive ping/pong |

All data on DATA and LASTDATA is encrypted with AES-ECB XOR stream cipher using the site's crypto key and the gateway's MAC address.

## 2. Authentication

1. Write `0x00` to AUTH characteristic (requests challenge)
2. Read AUTH → 16-byte random challenge
3. Compute response: `SHA256(key XOR challenge)` → split into two 16-byte halves → XOR them
4. Write 16-byte response to AUTH
5. Verify with ping: write random byte to PING, read back, expect `(byte + 1) & 0xFF`

## 3. Encryption

```
key = 16-byte site crypto key
addr = 6-byte gateway MAC (reversed byte order)
buf = addr + addr + addr[:4]  # 16 bytes
ct = AES-ECB(key, buf)
output[i] = data[i] XOR ct[i % 16]
```

Same function encrypts and decrypts (symmetric XOR stream).

## 4. Packet Format

All commands/events share this header:

```
AA VV TT CCCC [payload...]

AA   = Device address (1 byte)
VV   = Version, always 0x01
TT   = Command type (see below)
CCCC = Command ID (2 bytes, big-endian)
```

### Command Types (TT)

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | WRITE | Command, no response expected |
| 0x01 | ACK | Acknowledgment |
| 0x02 | READ | Request response |
| 0x10 | DONT_RESPOND | Command, explicitly no response |

Note: In practice, most outgoing commands use `0x01 0x10` (version=0x01, type=DONT_RESPOND).
Read requests use `0x01 0x02`.

### Special Addresses

| Address | Meaning |
|---------|---------|
| 0x00 | Broadcast (buttons, time set) |
| 0x01 | Time broadcast target |
| 0x02 | Scene trigger (convention) |
| 3-255 | Device addresses (assigned by cloud config) |

## 5. Commands — Complete Catalog

### 5.1 On/Off State (0x0097)

Turn device on or off without changing dim level.

```
Write: AA 0110 0097 SS
  SS = 0x00 (off) or 0x01 (on)

Event: AA 0110 0097 SS
  Same format for incoming state change notifications
```

### 5.2 Dim + State (0x0098 / 0x00C8)

Turn on/off with dim level. Two command IDs with same format:
- `0x0098` = Group output state + level
- `0x00C8` = Individual output state + level

```
Write: AA 0110 0098 SS DDDD
  SS   = 0x00 (off) or 0x01 (on)
  DDDD = Dim level (2 bytes). Second byte is the effective 0-255 level.
         Convention: send same byte twice (e.g., 0x80 0x80 for ~50%)

Event: AA 0110 00C8 SS DD1 DD2 [extra]
  DD2 = effective dim level (0-255)
  For covers: DD1,DD2 encode position (little-endian signed)
  extra byte may contain cover tilt angle (6-bit signed)
```

### 5.3 Scene Activation (0x0021)

```
Write: 02 0110 0021 II
  II = Scene index (from cloud config sceneIndex mapping)

Event: 00 0110 0021 II  (or 02 0110 0021 II)
  Broadcast when scene is activated
```

### 5.4 Button Press (0x0016)

```
Event: 00 0110 0016 AA BB [XX]
  AA = Source device address
  BB = Button number (0-3)
  XX = 0x01 press, 0x00 release (optional)

Button mapping for WPH-01 (4-button panel):
  0 = Top left, 1 = Top right, 2 = Bottom left, 3 = Bottom right
Button mapping for WRT-01 (rotary):
  0 = Rotary button press
```

### 5.5 Button Prepare / Event Subscribe (0x0015)

```
Write: 00 0110 0015
  Requests ALL buttons to send press events (not just battery-powered ones).
  Used by Plejd app for button identification. Also enables event notifications.
```

### 5.6 Time (0x001B)

```
Set time:
  Write: 00 0110 001B TTTTTTTTTT
  T = Unix timestamp, 5 bytes little-endian

Request time:
  Write: AA 0102 001B
  Response comes via LASTDATA notification:
  01 0110 001B TTTTTTTT 01
  T = Unix timestamp, 4 bytes little-endian
```

Note: pyplejd reads LASTDATA directly after requesting time, then parses bytes 5:9 as LE timestamp.

### 5.7 Color Temperature (0x0420 + MiniPkg 0x01)

New-style command using the OUTPUT_SET (0x0420) framework with MiniPkg sub-packets.

```
Write: AA 0110 0420 [MiniPkg: Source=0x08(app)] [MiniPkg: WhiteBalance=TTTT]

Legacy format (also observed):
  AA 0110 0420 03 01 11 TTTT
  03 = unknown/source byte
  TTTT = Color temperature in Kelvin (big-endian)
  Typical range: 2200-4000K (declared in device cloud data)
```

### 5.8 Cover Control (0x0420 + MiniPkg 0x07)

```
Set position:
  AA 0110 0420 [Source=APP] [WindowControl: 01 LLLL]
  LL = Position level (0-255, mapped from 0-100%)

Stop:
  AA 0110 0420 [Source=APP] [WindowControl: 00]

Legacy format:
  Set:  AA 0110 0420 03 08 27 01 PPPP  (PP = position, big-endian)
  Stop: AA 0110 0420 03 08 07 00
```

Cover events contain position + tilt data:
- State byte: 0=stopped, 1=moving
- Position: 7-bit value (0x00-0x7F) representing 0-100%
- Bit 7 of position byte: direction (1=up, 0=down)
- Tilt: 6-bit signed integer in 5° increments

### 5.9 Thermostat / Climate Control

#### Set Temperature Setpoint (0x045C)

```
Write: AA 0110 045C TTTT
  TT = Temperature × 10, little-endian (e.g., 225 = 22.5°C)

Event: AA 0110 045C ...payload...
  Bytes 5:7 = setpoint (little-endian)
```

#### Set Operating Mode (0x045F)

```
Write: AA 0110 045F MM
  MM = Mode:
    0 = Service (off)
    1 = Curing
    2 = Vacation
    3 = Boost
    4 = Frost Protection
    5 = Night Time Reduction
    6 = Day Time Reduction
    7 = Normal
```

#### PWM Duty (0x0461)

For PWM-regulated thermostats:
```
Write: AA 0110 0461 DD
  DD = Duty cycle (0-100%)
```

#### Reset Operating Mode (0x047E)

```
Write: AA 0110 047E
  Resets thermostat to normal mode
```

#### Thermostat State Encoding (via 0x0098/0x00C8)

State + dim bytes encode thermostat data as a 24-bit field:
```
Bits: 0000 000S MMME TTTT TTCC CCCC
  S = State bit
  M = Mode (3 bits)
  E = Error flag
  T = Target temperature (7 bits, value - 10 for TEMP mode)
  C = Current temperature (6 bits, value - 10 for TEMP mode)
  Extra byte bit 7: 1 = currently heating
```

### 5.10 Ambient Light Level (0x0434)

```
Read request: AA 0102 0434
  Request ambient light sensor reading from motion sensor
```

### 5.11 Motion Events

Motion sensors (WMS-01) send events via the 0x0420 OUTPUT_SET framework with MiniPkg sub-packets:

```
Event: AA 0110 0420 [Source=MOTION] [BatteryInfo=XXXX] [SenderDeviceType=XX] [Lux=XX]

Lux values: 0x01 = below limit (dark), 0x02 = above limit (bright)
Motion events are rate-limited to ~25-35 seconds
```

## 6. MiniPkg Sub-Packet Format

Used within OUTPUT_SET (0x0420) commands for extensible payloads.

```
Header byte: FSSS TTTT
  F    = Flag bit (0x80)
  SSS  = Payload size - 1 (3 bits, range 0-7 → size 1-8)
  TTTT = Type (4 bits, 0x0-0xE)
  
If TTTT = 0xF (15): Extended type, next byte = type - 15
```

### MiniPkg Types

| Type | Name | Description |
|------|------|-------------|
| 0x01 | WhiteBalance | Color temperature (2 bytes, big-endian Kelvin) |
| 0x03 | Source | Command source: 0x01=manual, 0x03=motion, 0x08=app |
| 0x06 | Lux | Light level: 0x01=dark, 0x02=bright |
| 0x07 | WindowControl | Cover control (position, stop) |
| 0x10 | Channel | Channel selector |
| 0x16 | BatteryInfo | Battery voltage (2 bytes, big-endian) |
| 0x18 | Tilt | Cover tilt angle (1 byte) |

## 7. LIGHTLEVEL Characteristic

Polling for current device states:

```
Write 0x01 to LIGHTLEVEL → triggers state report
Response: 10 or 20 bytes per device, subscribe for notifications

Per-device (10 bytes):
  Byte 0: Device address
  Byte 1: State (on/off)
  Bytes 2-4: Unknown
  Bytes 5-6: Dim level (little-endian, 0-65535)
  Bytes 7-9: Remaining payload (varies by device type)
```

For thermostats: parsed as state + payload for mode/target/current.
For covers: parsed as state + position/direction.

## 8. Cloud API

Base URL: `https://cloud.plejd.com`
App ID: `zHtVqXt8k4yFyk2QGmgp48D9xZr2G94xWYnF4dak`

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/parse/login` | Authenticate, returns sessionToken |
| POST | `/parse/functions/getSiteList` | List user's sites |
| POST | `/parse/functions/getSiteById?siteId=X` | Get full site details |

### Headers

```
X-Parse-Application-Id: zHtVqXt8k4yFyk2QGmgp48D9xZr2G94xWYnF4dak
X-Parse-Session-Token: <from login>
Content-Type: application/json
```

### Key Site Details Fields

- `plejdMesh.cryptoKey` — The BLE encryption key
- `deviceAddress` — Map of deviceId → BLE address (1-255)
- `outputAddress` — Map of deviceId → {output → address}
- `inputAddress` — Map of deviceId → {input → address}
- `rxAddress` — Map of deviceId → {output → rxAddress}
- `outputGroups` — Map of groups → device lists
- `sceneIndex` — Map of sceneId → scene index number
- `roomAddress` — Map of roomId → address

### Device Traits (bitfield)

| Bit | Value | Name |
|-----|-------|------|
| 0 | 0x01 | Groupable |
| 1 | 0x02 | Dimmable |
| 2 | 0x04 | WhiteTunable (color temp) |
| 3 | 0x08 | Powerable (on/off) |
| 4 | 0x10 | Coverable |
| 5 | 0x20 | ClimateControllable |
| 6 | 0x40 | CoverTiltable |
| 7 | 0x80 | ClimatePWM |

## 9. BLE Scanning & Connection

- Plejd devices advertise as `"P mesh XXXX"`
- Manufacturer data company ID: **887** (0x0377)
- MAC address extracted from manufacturer data bytes [4:10], reversed
- Connect to strongest RSSI device that has the POWER trait
- After connection: authenticate, then subscribe to LASTDATA and LIGHTLEVEL notifications

## 10. Keepalive / Health Check

- Periodic ping every ~5 seconds (justplejd)
- If ping fails: disconnect and reconnect to mesh
- Reconnection picks strongest available device

## 11. Known Device Types

From firmware notes in cloud data:

| Hardware ID | Type | Description |
|-------------|------|-------------|
| DIM-01 | Light | Dimmer module |
| DIM-02 | Light | Dimmer module (newer) |
| LED-10 | Light | LED driver |
| CTR-01 | Light | Controller |
| DWN-01 | Light | Downlight (can be grouped as fellowship follower) |
| REL-01 | Relay | On/off relay (no dimming) |
| REL-02 | Relay | On/off relay (newer) |
| SPR-01 | Relay | Smart plug |
| WPH-01 | Button | 4-button wireless panel (battery) |
| WRT-01 | Button | Wireless rotary (battery) |
| WMS-01 | Motion | Wireless motion sensor (battery) |
| EXT-01 | Relay | External relay (no POWER trait but still controllable) |
| BAT-01 | Other | Battery backup (keeps mesh clock after power loss) |
| GWY-01 | Other | Gateway (cloud connectivity) |
| DAL-01 | Light | DALI controller |
| TRM-01 | Climate | Thermostat (temperature or PWM regulation) |
| JAL-01 | Cover | Jalousie/blind controller |
| ROL-01 | Cover | Roller blind controller |

## 12. Missing / Unknown

Things observed but not fully understood:

1. **Broadcast semantics**: Addresses 0x00, 0x01, 0x02 have special meaning but the rules aren't fully clear
2. **MiniPkg flag bit**: The 0x80 flag in MiniPkg headers — purpose unknown
3. **Cover angle encoding**: Described as 6-bit signed integer in 5° increments but decoding is unreliable
4. **Button long press**: Holding buttons sends different commands (undocumented)
5. **DALI integration**: DAL-01 protocol extensions unknown
6. **Gateway commands**: GWY-01 may have additional cloud-bridge commands
7. **Firmware update protocol**: Not documented
8. **Group addressing**: How outputGroups map to mesh behavior
9. **rxAddress**: Exact purpose — possibly a "receive address" for group state updates

---

## 13. What justplejd-cli Currently Supports vs. What's Possible

### ✅ Implemented
- Cloud login + site details fetch
- BLE scanning, connection, authentication
- On/off (0x0097)
- Dim (0x0098)
- Scene activation (0x0021)
- Color temperature (0x0420 legacy format)
- Cover position + stop (0x0420 legacy format)
- Time get/set (0x001B)
- Listen for events (LASTDATA notifications)
- Event parsing (scene, button, dim, state, color temp, motion)
- Status polling (LIGHTLEVEL)

### 🔲 Not Yet Implemented
- **Thermostat control** (0x045C setpoint, 0x045F mode, 0x0461 PWM duty)
- **Thermostat state parsing** (24-bit packed mode/target/current)
- **MiniPkg-based commands** (new-style 0x0420 with proper sub-packet encoding)
- **Button event subscription** (0x0015 CMD_EVENT_PREPARE)
- **Motion sensor state** (ambient light reading via 0x0434)
- **Cover tilt control** (MiniPkg type 0x18)
- **LIGHTLEVEL parsing** (10-byte per-device state snapshots)
- **Reconnection/healthcheck** (auto-reconnect on disconnect)
- **Group addressing** (outputGroups from cloud config)
- **Fellowship follower awareness** (grouped DWN-01 devices)
