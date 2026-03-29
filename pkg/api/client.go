package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	DefaultBaseURL = "https://cloud.plejd.com"
	AppID          = "zHtVqXt8k4yFyk2QGmgp48D9xZr2G94xWYnF4dak"
)

// Client is a Plejd cloud API client.
type Client struct {
	BaseURL      string
	HTTPClient   *http.Client
	sessionToken string
}

// NewClient creates a new Plejd API client.
func NewClient() *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Parse-Application-Id", AppID)
	req.Header.Set("Content-Type", "application/json")
	if c.sessionToken != "" {
		req.Header.Set("X-Parse-Session-Token", c.sessionToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return data, resp.StatusCode, nil
}

// Login authenticates with the Plejd cloud API and stores the session token.
func (c *Client) Login(ctx context.Context, username, password string) error {
	body := map[string]string{"username": username, "password": password}
	data, status, err := c.doRequest(ctx, http.MethodPost, "/parse/login", body)
	if err != nil {
		return err
	}

	if status != http.StatusOK {
		var errResp struct {
			Code  int    `json:"code"`
			Error string `json:"error"`
		}
		json.Unmarshal(data, &errResp)
		if errResp.Code == 101 {
			return fmt.Errorf("invalid username/password")
		}
		return fmt.Errorf("login failed (HTTP %d): %s", status, string(data))
	}

	var result struct {
		SessionToken string `json:"sessionToken"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse login response: %w", err)
	}

	c.sessionToken = result.SessionToken
	return nil
}

// GetSites returns a list of sites for the authenticated user.
func (c *Client) GetSites(ctx context.Context) ([]Site, error) {
	if c.sessionToken == "" {
		return nil, fmt.Errorf("not logged in")
	}

	data, status, err := c.doRequest(ctx, http.MethodPost, "/parse/functions/getSiteList", map[string]string{})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("getSiteList failed (HTTP %d)", status)
	}

	var resp struct {
		Result []struct {
			Site struct {
				SiteID string `json:"siteId"`
				Title  string `json:"title"`
			} `json:"site"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse site list: %w", err)
	}

	sites := make([]Site, len(resp.Result))
	for i, r := range resp.Result {
		sites[i] = Site{
			SiteID: r.Site.SiteID,
			Title:  r.Site.Title,
		}
	}
	return sites, nil
}

// GetSiteDetails returns full site configuration including devices, rooms, scenes, and crypto key.
func (c *Client) GetSiteDetails(ctx context.Context, siteID string) (*Site, error) {
	if c.sessionToken == "" {
		return nil, fmt.Errorf("not logged in")
	}

	body := map[string]string{"siteId": siteID}
	data, status, err := c.doRequest(ctx, http.MethodPost, "/parse/functions/getSiteById", body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("getSiteById failed (HTTP %d)", status)
	}

	var resp struct {
		Result []json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse site details: %w", err)
	}
	if len(resp.Result) == 0 {
		return nil, fmt.Errorf("no site data returned")
	}

	var raw struct {
		Site struct {
			SiteID string `json:"siteId"`
			Title  string `json:"title"`
		} `json:"site"`
		PlejdMesh struct {
			CryptoKey string `json:"cryptoKey"`
		} `json:"plejdMesh"`
		Rooms []struct {
			RoomID string `json:"roomId"`
			Title  string `json:"title"`
		} `json:"rooms"`
		Devices []struct {
			DeviceID string `json:"deviceId"`
			Title    string `json:"title"`
			RoomID   string `json:"roomId"`
			Traits   int    `json:"traits"`
		} `json:"devices"`
		DeviceAddress map[string]int `json:"deviceAddress"`
		Scenes        []struct {
			SceneID string `json:"sceneId"`
			Title   string `json:"title"`
		} `json:"scenes"`
		SceneIndex map[string]int `json:"sceneIndex"`
	}
	if err := json.Unmarshal(resp.Result[0], &raw); err != nil {
		return nil, fmt.Errorf("parse site data: %w", err)
	}

	site := &Site{
		SiteID:    raw.Site.SiteID,
		Title:     raw.Site.Title,
		CryptoKey: raw.PlejdMesh.CryptoKey,
	}

	for _, r := range raw.Rooms {
		site.Rooms = append(site.Rooms, Room{
			RoomID: r.RoomID,
			Title:  r.Title,
		})
	}

	for _, d := range raw.Devices {
		site.Devices = append(site.Devices, Device{
			DeviceID: d.DeviceID,
			Title:    d.Title,
			RoomID:   d.RoomID,
			Traits:   Traits(d.Traits),
			Address:  raw.DeviceAddress[d.DeviceID],
		})
	}

	for _, s := range raw.Scenes {
		site.Scenes = append(site.Scenes, Scene{
			SceneID: s.SceneID,
			Title:   s.Title,
			Index:   raw.SceneIndex[s.SceneID],
		})
	}

	return site, nil
}
