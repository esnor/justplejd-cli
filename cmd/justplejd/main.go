package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/peterssonjesper/justplejd-cli/pkg/api"
	"github.com/peterssonjesper/justplejd-cli/pkg/ble"
	"github.com/peterssonjesper/justplejd-cli/pkg/protocol"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	verbose    bool
	jsonOutput bool
	timeout    time.Duration
)

func main() {
	root := &cobra.Command{
		Use:   "justplejd",
		Short: "Control Plejd smart home devices over BLE",
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/justplejd/config.json)")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON output format")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 3*time.Second, "BLE scan timeout")

	root.AddCommand(
		loginCmd(),
		sitesCmd(),
		devicesCmd(),
		roomsCmd(),
		scenesCmd(),
		onCmd(),
		offCmd(),
		dimCmd(),
		sceneCmd(),
		listenCmd(),
		statusCmd(),
		timeCmd(),
		thermostatCmd(),
		tiltCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return defaultConfigPath()
}

func mustLoadConfig() *Config {
	cfg, err := loadConfig(getConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func mustHaveSite(cfg *Config) *api.Site {
	if cfg.Site == nil {
		fmt.Fprintln(os.Stderr, "No site configured. Run 'justplejd login' first.")
		os.Exit(1)
	}
	return cfg.Site
}

func resolveDevice(site *api.Site, name string) (*api.Device, error) {
	// Try by address (numeric)
	if addr, err := strconv.Atoi(name); err == nil {
		for i, d := range site.Devices {
			if d.Address == addr {
				return &site.Devices[i], nil
			}
		}
	}

	// Try by name (case-insensitive)
	lower := strings.ToLower(name)
	for i, d := range site.Devices {
		if strings.ToLower(d.Title) == lower {
			return &site.Devices[i], nil
		}
	}

	return nil, fmt.Errorf("device %q not found", name)
}

func resolveScene(site *api.Site, name string) (*api.Scene, error) {
	// Try by index
	if idx, err := strconv.Atoi(name); err == nil {
		for i, s := range site.Scenes {
			if s.Index == idx {
				return &site.Scenes[i], nil
			}
		}
	}

	// Try by name
	lower := strings.ToLower(name)
	for i, s := range site.Scenes {
		if strings.ToLower(s.Title) == lower {
			return &site.Scenes[i], nil
		}
	}

	return nil, fmt.Errorf("scene %q not found", name)
}

func connectBLE(cfg *Config) (*ble.PlejdBLE, error) {
	adapter, err := newBLEAdapter()
	if err != nil {
		return nil, fmt.Errorf("init BLE: %w", err)
	}

	p := ble.NewPlejdBLE(adapter, cfg.CryptoKey)

	// Build list of gateway-capable device IDs
	var gatewayIDs []string
	for _, d := range cfg.Site.Devices {
		if d.Traits.HasPower() {
			gatewayIDs = append(gatewayIDs, d.DeviceID)
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Scanning for Plejd devices (%d gateway candidates)...\n", len(gatewayIDs))
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	devices, err := p.ScanForDevices(ctx, gatewayIDs)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no Plejd devices found nearby")
	}

	gateway := devices[0]
	if verbose {
		fmt.Fprintf(os.Stderr, "Connecting to %s (RSSI %d)...\n", gateway.MACAddress, gateway.RSSI)
	}

	if err := p.Connect(gateway); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	ok, err := p.Ping()
	if err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("ping failed — authentication may have failed")
	}

	if verbose {
		fmt.Fprintln(os.Stderr, "Connected and authenticated.")
	}

	return p, nil
}

func printJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

// loginCmd authenticates with the Plejd cloud and stores config.
func loginCmd() *cobra.Command {
	var email, password, siteID string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Plejd cloud API",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient()
			ctx := context.Background()

			if verbose {
				fmt.Fprintln(os.Stderr, "Logging in...")
			}

			if err := client.Login(ctx, email, password); err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			sites, err := client.GetSites(ctx)
			if err != nil {
				return fmt.Errorf("get sites: %w", err)
			}

			if len(sites) == 0 {
				return fmt.Errorf("no sites found on this account")
			}

			// Pick site
			targetSiteID := siteID
			if targetSiteID == "" {
				if len(sites) > 1 {
					fmt.Fprintln(os.Stderr, "Multiple sites found. Use --site-id to select one:")
					for _, s := range sites {
						fmt.Fprintf(os.Stderr, "  %s  %s\n", s.SiteID, s.Title)
					}
					return fmt.Errorf("multiple sites found, specify --site-id")
				}
				targetSiteID = sites[0].SiteID
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "Fetching site details for %s...\n", targetSiteID)
			}

			site, err := client.GetSiteDetails(ctx, targetSiteID)
			if err != nil {
				return fmt.Errorf("get site details: %w", err)
			}

			cfg := &Config{
				Email:     email,
				SiteID:    site.SiteID,
				CryptoKey: site.CryptoKey,
				Site:      site,
			}

			if err := saveConfig(getConfigPath(), cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			if jsonOutput {
				printJSON(site)
			} else {
				fmt.Printf("Logged in to %q (%d devices, %d rooms, %d scenes)\n",
					site.Title, len(site.Devices), len(site.Rooms), len(site.Scenes))
				fmt.Printf("Config saved to %s\n", getConfigPath())
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Plejd account email")
	cmd.Flags().StringVar(&password, "password", "", "Plejd account password")
	cmd.Flags().StringVar(&siteID, "site-id", "", "Site ID (required if multiple sites)")
	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("password")

	return cmd
}

// sitesCmd lists all sites on the account.
func sitesCmd() *cobra.Command {
	var email, password string

	cmd := &cobra.Command{
		Use:   "sites",
		Short: "List all sites on the account",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient()
			ctx := context.Background()

			if err := client.Login(ctx, email, password); err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			sites, err := client.GetSites(ctx)
			if err != nil {
				return fmt.Errorf("get sites: %w", err)
			}

			if len(sites) == 0 {
				return fmt.Errorf("no sites found on this account")
			}

			if jsonOutput {
				printJSON(sites)
			} else {
				for _, s := range sites {
					fmt.Printf("  %-36s  %s\n", s.SiteID, s.Title)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Plejd account email")
	cmd.Flags().StringVar(&password, "password", "", "Plejd account password")
	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("password")

	return cmd
}

// devicesCmd lists all configured devices.
func devicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: "List all devices with room, address, and traits",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			if jsonOutput {
				printJSON(site.Devices)
				return nil
			}

			roomMap := make(map[string]string)
			for _, r := range site.Rooms {
				roomMap[r.RoomID] = r.Title
			}

			for _, d := range site.Devices {
				room := roomMap[d.RoomID]
				if room == "" {
					room = "(no room)"
				}
				traits := formatTraits(d.Traits)
				fmt.Printf("  %-20s addr=%-3d room=%-15s traits=%s\n",
					d.Title, d.Address, room, traits)
			}
			return nil
		},
	}
}

func formatTraits(t api.Traits) string {
	var parts []string
	if t&api.TraitGroup != 0 {
		parts = append(parts, "GROUP")
	}
	if t&api.TraitDim != 0 {
		parts = append(parts, "DIM")
	}
	if t&api.TraitTemp != 0 {
		parts = append(parts, "TEMP")
	}
	if t&api.TraitPower != 0 {
		parts = append(parts, "POWER")
	}
	if t&api.TraitCover != 0 {
		parts = append(parts, "COVER")
	}
	if t&api.TraitClimate != 0 {
		parts = append(parts, "CLIMATE")
	}
	if t&api.TraitTilt != 0 {
		parts = append(parts, "TILT")
	}
	if t&api.TraitClimatePWM != 0 {
		parts = append(parts, "CLIMATE_PWM")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}

// roomsCmd lists rooms and their devices.
func roomsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rooms",
		Short: "List rooms and their devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			if jsonOutput {
				printJSON(site.Rooms)
				return nil
			}

			for _, r := range site.Rooms {
				fmt.Printf("%s\n", r.Title)
				for _, d := range site.Devices {
					if d.RoomID == r.RoomID {
						fmt.Printf("  - %s (addr=%d)\n", d.Title, d.Address)
					}
				}
			}
			return nil
		},
	}
}

// scenesCmd lists scenes.
func scenesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scenes",
		Short: "List scenes with indices",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			if jsonOutput {
				printJSON(site.Scenes)
				return nil
			}

			for _, s := range site.Scenes {
				fmt.Printf("  %-20s index=%d\n", s.Title, s.Index)
			}
			return nil
		},
	}
}

// onCmd turns a device on.
func onCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "on DEVICE",
		Short: "Turn on a device (by name or address)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			dev, err := resolveDevice(site, args[0])
			if err != nil {
				return err
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.TurnOn(byte(dev.Address))
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if jsonOutput {
				printJSON(map[string]interface{}{"device": dev.Title, "action": "on", "address": dev.Address})
			} else {
				fmt.Printf("Turned on %s (addr=%d)\n", dev.Title, dev.Address)
			}
			return nil
		},
	}
}

// offCmd turns a device off.
func offCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "off DEVICE",
		Short: "Turn off a device (by name or address)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			dev, err := resolveDevice(site, args[0])
			if err != nil {
				return err
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.TurnOff(byte(dev.Address))
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if jsonOutput {
				printJSON(map[string]interface{}{"device": dev.Title, "action": "off", "address": dev.Address})
			} else {
				fmt.Printf("Turned off %s (addr=%d)\n", dev.Title, dev.Address)
			}
			return nil
		},
	}
}

// dimCmd sets dim level on a device.
func dimCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dim DEVICE LEVEL",
		Short: "Set dim level (0-255 or 0-100%)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			dev, err := resolveDevice(site, args[0])
			if err != nil {
				return err
			}

			levelStr := args[1]
			var level int
			if strings.HasSuffix(levelStr, "%") {
				pct, err := strconv.Atoi(strings.TrimSuffix(levelStr, "%"))
				if err != nil || pct < 0 || pct > 100 {
					return fmt.Errorf("invalid percentage: %s", levelStr)
				}
				level = pct * 255 / 100
			} else {
				level, err = strconv.Atoi(levelStr)
				if err != nil || level < 0 || level > 255 {
					return fmt.Errorf("invalid level: %s (must be 0-255 or 0-100%%)", levelStr)
				}
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.Dim(byte(dev.Address), byte(level))
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if jsonOutput {
				printJSON(map[string]interface{}{"device": dev.Title, "action": "dim", "level": level, "address": dev.Address})
			} else {
				fmt.Printf("Set %s to dim level %d (addr=%d)\n", dev.Title, level, dev.Address)
			}
			return nil
		},
	}
}

// sceneCmd activates a scene.
func sceneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scene SCENE",
		Short: "Activate scene (by name or index)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			sc, err := resolveScene(site, args[0])
			if err != nil {
				return err
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.ActivateScene(byte(sc.Index))
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if jsonOutput {
				printJSON(map[string]interface{}{"scene": sc.Title, "action": "activate", "index": sc.Index})
			} else {
				fmt.Printf("Activated scene %q (index=%d)\n", sc.Title, sc.Index)
			}
			return nil
		},
	}
}

// listenCmd listens for mesh events.
func listenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "listen",
		Short: "Listen for mesh events (real-time)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			mustHaveSite(cfg)

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			// Enable button events from all devices
			if err := p.RequestButtonEvents(); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: failed to enable button events: %v\n", err)
				}
			}

			fmt.Fprintln(os.Stderr, "Listening for events (Ctrl+C to stop)...")

			// Subscribe to LASTDATA (encrypted mesh events)
			if err := p.Listen(func(ev protocol.Event) {
				if jsonOutput {
					data, _ := json.Marshal(ev)
					fmt.Println(string(data))
				} else {
					printEvent(ev)
				}
			}); err != nil {
				return fmt.Errorf("listen: %w", err)
			}

			// Subscribe to LIGHTLEVEL notifications
			if err := p.SubscribeLightLevel(func(states []protocol.DeviceState) {
				for _, s := range states {
					if jsonOutput {
						data, _ := json.Marshal(s)
						fmt.Println(string(data))
					} else {
						state := "OFF"
						if s.State != 0 {
							state = "ON"
						}
						fmt.Printf("[LIGHTLEVEL] addr=%d %s dim=%d\n", s.Address, state, s.DimLevel)
					}
				}
			}); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: failed to subscribe LIGHTLEVEL: %v\n", err)
				}
			}

			// Wait for interrupt
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			<-ctx.Done()

			return nil
		},
	}
}

func printEvent(ev protocol.Event) {
	switch ev.Type {
	case protocol.EventSceneActivated:
		fmt.Printf("[SCENE] Scene %d activated\n", ev.Scene)
	case protocol.EventButtonPress:
		fmt.Printf("[BUTTON] addr=%d button=%d action=%s\n", ev.Address, ev.Button, ev.Action)
	case protocol.EventDim:
		fmt.Printf("[DIM] addr=%d state=%d dim=%d\n", ev.Address, ev.State, ev.Dim)
	case protocol.EventStateChange:
		state := "OFF"
		if ev.State != 0 {
			state = "ON"
		}
		fmt.Printf("[STATE] addr=%d %s\n", ev.Address, state)
	case protocol.EventColorTemp:
		fmt.Printf("[COLORTEMP] addr=%d temp=%dK\n", ev.Address, ev.Temperature)
	case protocol.EventMotion:
		fmt.Printf("[MOTION] addr=%d lightlevel=%d\n", ev.Address, ev.LightLevel)
	case protocol.EventThermostatState:
		modeName := protocol.ThermostatModeName[ev.ThermoMode]
		heating := ""
		if ev.ThermoHeating {
			heating = " HEATING"
		}
		errStr := ""
		if ev.ThermoError {
			errStr = " ERROR"
		}
		fmt.Printf("[THERMOSTAT] addr=%d mode=%s target=%.1f°C current=%.1f°C%s%s\n",
			ev.Address, modeName, ev.ThermoTarget, ev.ThermoCurrent, heating, errStr)
	case protocol.EventThermostatSet:
		fmt.Printf("[THERMOSTAT_SET] addr=%d target=%.1f°C\n", ev.Address, ev.ThermoTarget)
	case protocol.EventCoverState:
		dir := "down"
		if ev.CoverDirection == 1 {
			dir = "up"
		}
		moving := "stopped"
		if ev.CoverMoving {
			moving = "moving"
		}
		fmt.Printf("[COVER] addr=%d pos=%d%% dir=%s tilt=%d %s\n",
			ev.Address, ev.CoverPosition, dir, ev.CoverTilt, moving)
	case protocol.EventTimeResponse:
		t := time.Unix(ev.Timestamp, 0)
		fmt.Printf("[TIME] addr=%d %s\n", ev.Address, t.Format(time.RFC3339))
	default:
		fmt.Printf("[UNKNOWN] %+v\n", ev)
	}
}

// statusCmd shows connection status and optionally polls device states.
func statusCmd() *cobra.Command {
	var poll bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show connection status and device states",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()

			if cfg.Site == nil {
				if jsonOutput {
					printJSON(map[string]interface{}{"configured": false})
				} else {
					fmt.Println("Not configured. Run 'justplejd login' first.")
				}
				return nil
			}

			site := cfg.Site

			if !poll {
				if jsonOutput {
					printJSON(map[string]interface{}{
						"configured": true,
						"site":       site.Title,
						"site_id":    site.SiteID,
						"devices":    len(site.Devices),
						"rooms":      len(site.Rooms),
						"scenes":     len(site.Scenes),
					})
				} else {
					fmt.Printf("Site: %s\n", site.Title)
					fmt.Printf("Site ID: %s\n", site.SiteID)
					fmt.Printf("Devices: %d\n", len(site.Devices))
					fmt.Printf("Rooms: %d\n", len(site.Rooms))
					fmt.Printf("Scenes: %d\n", len(site.Scenes))
					fmt.Printf("Config: %s\n", getConfigPath())
				}
				return nil
			}

			// Poll LIGHTLEVEL for live device states
			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			states, err := p.PollLightLevel()
			if err != nil {
				return fmt.Errorf("poll lightlevel: %w", err)
			}

			// Build address->name map
			nameMap := make(map[int]string)
			for _, d := range site.Devices {
				nameMap[d.Address] = d.Title
			}

			if jsonOutput {
				printJSON(states)
			} else {
				if len(states) == 0 {
					fmt.Println("No device states received.")
					return nil
				}
				for _, s := range states {
					name := nameMap[s.Address]
					if name == "" {
						name = fmt.Sprintf("addr=%d", s.Address)
					}
					state := "OFF"
					if s.State != 0 {
						state = "ON"
					}
					pct := s.DimLevel * 100 / 65535
					fmt.Printf("  %-20s %s  dim=%d%%\n", name, state, pct)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&poll, "poll", false, "Poll LIGHTLEVEL for live device states")
	return cmd
}

// timeCmd gets or sets the mesh time.
func timeCmd() *cobra.Command {
	var setTime bool

	cmd := &cobra.Command{
		Use:   "time",
		Short: "Get or set mesh time",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			mustHaveSite(cfg)

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			if setTime {
				now := time.Now().Unix()
				payload := protocol.SetTime(now)
				if err := p.SendCommand(payload); err != nil {
					return fmt.Errorf("set time: %w", err)
				}
				if jsonOutput {
					printJSON(map[string]interface{}{"action": "set_time", "timestamp": now})
				} else {
					fmt.Printf("Set mesh time to %s\n", time.Unix(now, 0).Format(time.RFC3339))
				}
			} else {
				// Request time from first device with POWER trait
				var addr byte
				for _, d := range cfg.Site.Devices {
					if d.Traits.HasPower() {
						addr = byte(d.Address)
						break
					}
				}
				payload := protocol.RequestTime(addr)
				if err := p.SendCommand(payload); err != nil {
					return fmt.Errorf("request time: %w", err)
				}
				if !jsonOutput {
					fmt.Printf("Requested time from device at addr=%d\n", addr)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&setTime, "set", false, "Set mesh time to current time")
	return cmd
}

// thermostatCmd manages thermostat devices.
func thermostatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thermostat",
		Short: "Control thermostat devices",
	}

	cmd.AddCommand(
		thermostatSetTempCmd(),
		thermostatModeCmd(),
		thermostatPWMCmd(),
	)

	return cmd
}

func thermostatSetTempCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-temp DEVICE TEMP",
		Short: "Set thermostat temperature setpoint (e.g., 22.5)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			dev, err := resolveDevice(site, args[0])
			if err != nil {
				return err
			}

			temp, err := strconv.ParseFloat(args[1], 64)
			if err != nil || temp < 5 || temp > 40 {
				return fmt.Errorf("invalid temperature: %s (must be 5-40°C)", args[1])
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.ThermostatSetTemp(byte(dev.Address), temp)
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if jsonOutput {
				printJSON(map[string]interface{}{"device": dev.Title, "action": "set_temp", "temperature": temp, "address": dev.Address})
			} else {
				fmt.Printf("Set %s to %.1f°C (addr=%d)\n", dev.Title, temp, dev.Address)
			}
			return nil
		},
	}
}

func thermostatModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mode DEVICE MODE",
		Short: "Set thermostat mode (normal/vacation/boost/frost/night/day/service/curing)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			dev, err := resolveDevice(site, args[0])
			if err != nil {
				return err
			}

			modeMap := map[string]byte{
				"service":  0,
				"curing":   1,
				"vacation": 2,
				"boost":    3,
				"frost":    4,
				"night":    5,
				"day":      6,
				"normal":   7,
			}

			mode, ok := modeMap[strings.ToLower(args[1])]
			if !ok {
				// Try numeric
				n, err := strconv.Atoi(args[1])
				if err != nil || n < 0 || n > 7 {
					return fmt.Errorf("invalid mode: %s (use: normal, vacation, boost, frost, night, day, service, curing, or 0-7)", args[1])
				}
				mode = byte(n)
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.ThermostatMode(byte(dev.Address), mode)
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			modeName := protocol.ThermostatModeName[int(mode)]
			if jsonOutput {
				printJSON(map[string]interface{}{"device": dev.Title, "action": "set_mode", "mode": modeName, "address": dev.Address})
			} else {
				fmt.Printf("Set %s to mode %s (addr=%d)\n", dev.Title, modeName, dev.Address)
			}
			return nil
		},
	}
}

func thermostatPWMCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pwm DEVICE DUTY",
		Short: "Set thermostat PWM duty cycle (0-100%)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			dev, err := resolveDevice(site, args[0])
			if err != nil {
				return err
			}

			duty, err := strconv.Atoi(args[1])
			if err != nil || duty < 0 || duty > 100 {
				return fmt.Errorf("invalid duty: %s (must be 0-100)", args[1])
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.ThermostatPWM(byte(dev.Address), byte(duty))
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if jsonOutput {
				printJSON(map[string]interface{}{"device": dev.Title, "action": "set_pwm", "duty": duty, "address": dev.Address})
			} else {
				fmt.Printf("Set %s PWM duty to %d%% (addr=%d)\n", dev.Title, duty, dev.Address)
			}
			return nil
		},
	}
}

// tiltCmd sets cover tilt angle.
func tiltCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tilt DEVICE ANGLE",
		Short: "Set cover tilt angle (0-255)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			site := mustHaveSite(cfg)

			dev, err := resolveDevice(site, args[0])
			if err != nil {
				return err
			}

			angle, err := strconv.Atoi(args[1])
			if err != nil || angle < 0 || angle > 255 {
				return fmt.Errorf("invalid angle: %s (must be 0-255)", args[1])
			}

			p, err := connectBLE(cfg)
			if err != nil {
				return err
			}
			defer p.Disconnect()

			payload := protocol.CoverTilt(byte(dev.Address), byte(angle))
			if err := p.SendCommand(payload); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if jsonOutput {
				printJSON(map[string]interface{}{"device": dev.Title, "action": "tilt", "angle": angle, "address": dev.Address})
			} else {
				fmt.Printf("Set %s tilt to %d (addr=%d)\n", dev.Title, angle, dev.Address)
			}
			return nil
		},
	}
}
