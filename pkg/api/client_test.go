package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/parse/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check app ID header
		if r.Header.Get("X-Parse-Application-Id") != AppID {
			http.Error(w, "missing app id", http.StatusUnauthorized)
			return
		}

		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if body.Username == "test@example.com" && body.Password == "secret123" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sessionToken": "test-session-token",
				"objectId":     "user123",
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":  101,
			"error": "Invalid username/password.",
		})
	})

	mux.HandleFunc("/parse/functions/getSiteList", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Parse-Session-Token") != "test-session-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"site": map[string]interface{}{
						"siteId": "site-uuid-1",
						"title":  "Home",
					},
					"plejdDevice": []string{"dev1", "dev2"},
				},
			},
		})
	})

	mux.HandleFunc("/parse/functions/getSiteById", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Parse-Session-Token") != "test-session-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"site": map[string]interface{}{
						"siteId": "site-uuid-1",
						"title":  "Home",
					},
					"plejdMesh": map[string]interface{}{
						"cryptoKey": "0123456789ABCDEF0123456789ABCDEF",
					},
					"rooms": []map[string]interface{}{
						{"roomId": "room-1", "title": "Kitchen"},
						{"roomId": "room-2", "title": "Living Room"},
					},
					"devices": []map[string]interface{}{
						{
							"deviceId": "dev-aabb",
							"title":    "Ceiling Light",
							"roomId":   "room-1",
							"traits":   10, // DIM | POWER
						},
						{
							"deviceId": "dev-ccdd",
							"title":    "Wall Switch",
							"roomId":   "room-2",
							"traits":   8, // POWER
						},
					},
					"deviceAddress": map[string]interface{}{
						"dev-aabb": 39,
						"dev-ccdd": 12,
					},
					"scenes": []map[string]interface{}{
						{"sceneId": "scene-1", "title": "Movie"},
						{"sceneId": "scene-2", "title": "Dinner"},
					},
					"sceneIndex": map[string]interface{}{
						"scene-1": 1,
						"scene-2": 2,
					},
				},
			},
		})
	})

	return httptest.NewServer(mux)
}

func TestLoginSuccess(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	err := c.Login(context.Background(), "test@example.com", "secret123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	err := c.Login(context.Background(), "wrong@example.com", "wrong")
	if err == nil {
		t.Fatal("expected error for invalid credentials")
	}
}

func TestGetSites(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	err := c.Login(context.Background(), "test@example.com", "secret123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	sites, err := c.GetSites(context.Background())
	if err != nil {
		t.Fatalf("GetSites failed: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(sites))
	}
	if sites[0].SiteID != "site-uuid-1" {
		t.Errorf("site ID: got %q, want %q", sites[0].SiteID, "site-uuid-1")
	}
	if sites[0].Title != "Home" {
		t.Errorf("site title: got %q, want %q", sites[0].Title, "Home")
	}
}

func TestGetSiteDetails(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	err := c.Login(context.Background(), "test@example.com", "secret123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	site, err := c.GetSiteDetails(context.Background(), "site-uuid-1")
	if err != nil {
		t.Fatalf("GetSiteDetails failed: %v", err)
	}
	if site == nil {
		t.Fatal("expected site, got nil")
	}

	if site.CryptoKey != "0123456789ABCDEF0123456789ABCDEF" {
		t.Errorf("crypto key: got %q", site.CryptoKey)
	}

	if len(site.Rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(site.Rooms))
	}
	if site.Rooms[0].Title != "Kitchen" {
		t.Errorf("room 0 title: got %q", site.Rooms[0].Title)
	}

	if len(site.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(site.Devices))
	}
	if site.Devices[0].Title != "Ceiling Light" {
		t.Errorf("device 0 title: got %q", site.Devices[0].Title)
	}
	if site.Devices[0].Address != 39 {
		t.Errorf("device 0 address: got %d, want 39", site.Devices[0].Address)
	}
	if !site.Devices[0].Traits.HasPower() {
		t.Error("device 0 should have POWER trait")
	}

	if len(site.Scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(site.Scenes))
	}
	if site.Scenes[0].Title != "Movie" {
		t.Errorf("scene 0 title: got %q", site.Scenes[0].Title)
	}
	if site.Scenes[0].Index != 1 {
		t.Errorf("scene 0 index: got %d, want 1", site.Scenes[0].Index)
	}
}

func TestGetSitesWithoutLogin(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	// Should fail without login
	_, err := c.GetSites(context.Background())
	if err == nil {
		t.Fatal("expected error without login")
	}
}
