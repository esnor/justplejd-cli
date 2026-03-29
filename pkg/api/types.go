package api

// Traits is a bitmask of device capabilities.
type Traits int

const (
	TraitGroup      Traits = 0x01
	TraitDim        Traits = 0x02
	TraitTemp       Traits = 0x04
	TraitPower      Traits = 0x08
	TraitCover      Traits = 0x10
	TraitClimate    Traits = 0x20
	TraitTilt       Traits = 0x40
	TraitClimatePWM Traits = 0x80
)

// HasPower returns true if the device has a power output (needed for gateway selection).
func (t Traits) HasPower() bool {
	return t&TraitPower != 0
}

// Device represents a Plejd device from the cloud API.
type Device struct {
	DeviceID string `json:"deviceId"`
	Title    string `json:"title"`
	RoomID   string `json:"roomId"`
	Traits   Traits `json:"traits"`
	Address  int    `json:"address"`
}

// Room represents a room from the cloud API.
type Room struct {
	RoomID string `json:"roomId"`
	Title  string `json:"title"`
}

// Scene represents a scene from the cloud API.
type Scene struct {
	SceneID string `json:"sceneId"`
	Title   string `json:"title"`
	Index   int    `json:"index"`
}

// Site represents a Plejd site with all configuration data.
type Site struct {
	SiteID    string   `json:"siteId"`
	Title     string   `json:"title"`
	CryptoKey string   `json:"cryptoKey"`
	Rooms     []Room   `json:"rooms"`
	Devices   []Device `json:"devices"`
	Scenes    []Scene  `json:"scenes"`
}
