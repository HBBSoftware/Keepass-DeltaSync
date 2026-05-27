// SPDX-License-Identifier: GPL-3.0-or-later

// Package api er den tynde HTTP-klient mod Delta-Sync-serveren.
// Hver eksporteret metode svarer 1:1 til et server-endpoint.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	userAgent      = "keepass-deltasync-client/0.1.0"
)

// Client er en HTTP-klient bundet til en konkret server-URL.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returnerer en klient mod baseURL. Tail-slashes trimmes så stien
// kan sammensættes uden dobbelt-slash.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// APIError er en typed fejl med server-leveret kode + besked.
type APIError struct {
	StatusCode int
	Code       string // "unauthorized", "invalid_token", ...
	Message    string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("server returned %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("server returned %d (%s): %s", e.StatusCode, e.Code, e.Message)
}

// EnrollResponse er det server-svar enroll-endpointet returnerer ved 201.
type EnrollResponse struct {
	Device struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		EnrolledAt string `json:"enrolled_at"`
	} `json:"device"`
	Token string `json:"token"`
}

// Enroll bytter en enrollment-token til en permanent device-token via
// POST /api/v1/devices/enroll. deviceName er valgfrit (sendes hvis non-tom).
func (c *Client) Enroll(ctx context.Context, enrollmentToken, deviceName string) (*EnrollResponse, error) {
	body := map[string]any{}
	if deviceName != "" {
		body["device_name"] = deviceName
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+"/api/v1/devices/enroll",
		bytes.NewReader(buf),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+enrollmentToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post enroll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp)
	}

	var out EnrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.Token == "" {
		return nil, errors.New("server returned empty device token")
	}
	return &out, nil
}

// User er bruger-info som returneret af /me.
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

// Device er device-info som returneret af /me.
type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EnrolledAt string `json:"enrolled_at"`
	LastSeen   string `json:"last_seen"`
}

// MeResponse er det fulde svar fra /me.
type MeResponse struct {
	User   User   `json:"user"`
	Device Device `json:"device"`
}

// Me kalder GET /api/v1/me med device-tokenet og returnerer bruger + device.
// Server-side opdaterer last_seen ved succesfuld auth.
func (c *Client) Me(ctx context.Context, deviceToken string) (*MeResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get me: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var out MeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// DeviceListEntry er én række i svaret fra GET /devices. Felterne overlapper
// med Device men inkluderer is_current — adskilt type, så Device kan bruges
// af endpoints (som /me) der ikke har det felt.
type DeviceListEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EnrolledAt string `json:"enrolled_at"`
	LastSeen   string `json:"last_seen"`
	IsCurrent  bool   `json:"is_current"`
}

// ListDevices kalder GET /api/v1/devices og returnerer alle devices for den
// bruger token'en hører til. Sorteret af serveren med nyeste enrollment først.
func (c *Client) ListDevices(ctx context.Context, deviceToken string) ([]DeviceListEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/devices", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get devices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var out struct {
		Devices []DeviceListEntry `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.Devices, nil
}

// LogEntry er én række i den autentificerede brugers audit-log.
// `Details` er fri-form JSON og afhænger af event_type — gemmes raw så
// klienten kan vise det uden at kende alle skemaer.
type LogEntry struct {
	OccurredAt string          `json:"occurred_at"`
	Level      string          `json:"level"`
	EventType  string          `json:"event_type"`
	IPAddress  string          `json:"ip_address"`
	UserAgent  string          `json:"user_agent"`
	DatabaseID *string         `json:"database_id"`
	EntryUUID  *string         `json:"entry_uuid"`
	Details    json.RawMessage `json:"details"`
	Success    bool            `json:"success"`
}

// ListLog kalder GET /api/v1/log med valgfri since-tid og limit. since er
// null hvis intet filter ønskes; serveren accepterer ISO 8601 med tidszone.
// limit clampes server-side til [1, 200] med default 50.
func (c *Client) ListLog(ctx context.Context, deviceToken string, since *time.Time, limit int) ([]LogEntry, error) {
	u := c.baseURL + "/api/v1/log"
	q := url.Values{}
	if since != nil {
		q.Set("since", since.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get log: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var out struct {
		Log   []LogEntry `json:"log"`
		Count int        `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.Log, nil
}

// Database er én database registreret hos serveren (returneret af
// /databases-endpoints).
type Database struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// CreateDatabase registrerer en ny database hos serveren og returnerer den
// server-genererede UUID (Database.ID). Navnet er fri-form 1-200 chars.
func (c *Client) CreateDatabase(ctx context.Context, deviceToken, name string) (*Database, error) {
	buf, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+"/api/v1/databases",
		bytes.NewReader(buf),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post databases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp)
	}

	var out struct {
		Database Database `json:"database"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.Database.ID == "" {
		return nil, errors.New("server returned empty database id")
	}
	return &out.Database, nil
}

// ListDatabases returnerer alle databaser registreret hos serveren for den
// bruger token'en hører til.
func (c *Client) ListDatabases(ctx context.Context, deviceToken string) ([]Database, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/databases", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get databases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var out struct {
		Databases []Database `json:"databases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.Databases, nil
}

// parseError læser body og bygger en APIError. Hvis JSON-parsing fejler bruges
// raw body som besked, så vi ikke skjuler nyttig diagnostik.
func parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && (payload.Error != "" || payload.Message != "") {
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       payload.Error,
			Message:    payload.Message,
		}
	}
	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    strings.TrimSpace(string(body)),
	}
}
