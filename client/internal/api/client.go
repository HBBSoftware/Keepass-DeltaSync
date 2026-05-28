// SPDX-License-Identifier: GPL-3.0-or-later

// Package api er den tynde HTTP-klient mod Delta-Sync-serveren.
// Hver eksporteret metode svarer 1:1 til et server-endpoint.
package api

import (
	"bytes"
	"context"
	"encoding/base64"
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
// publicKey er enhedens X25519 public-key til v2-sharing (base64-encoded i
// JSON); må være nil for legacy-flow, men v2-klienter bør altid sende det.
func (c *Client) Enroll(ctx context.Context, enrollmentToken, deviceName string, publicKey []byte) (*EnrollResponse, error) {
	body := map[string]any{}
	if deviceName != "" {
		body["device_name"] = deviceName
	}
	if publicKey != nil {
		body["public_key"] = base64.StdEncoding.EncodeToString(publicKey)
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

// Device er device-info som returneret af /me. PublicKey er en nullable
// base64-streng (32-byte X25519); NULL for legacy-enheder enrolled før v2.
type Device struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	EnrolledAt string  `json:"enrolled_at"`
	LastSeen   string  `json:"last_seen"`
	PublicKey  *string `json:"public_key,omitempty"`
}

// MeResponse er det fulde svar fra /me.
type MeResponse struct {
	User   User   `json:"user"`
	Device Device `json:"device"`
}

// UpdateDevicePublicKey opdaterer den nuværende enheds public_key via
// PATCH /api/v1/me. Bruges af auto-upgrade-flowet til at sætte public_key
// på legacy-enheder enrolled før v2.
func (c *Client) UpdateDevicePublicKey(ctx context.Context, deviceToken string, publicKey []byte) error {
	if len(publicKey) != 32 {
		return fmt.Errorf("public key must be 32 bytes, got %d", len(publicKey))
	}
	buf, err := json.Marshal(map[string]string{
		"public_key": base64.StdEncoding.EncodeToString(publicKey),
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/api/v1/me", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("patch me: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}
	return nil
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
// /databases-endpoints). Role er "owner" eller "member" og angiver hvilken
// ACL caller'en har. WrappedMasterKey er base64-encoded sealed-box krypteret
// til vores enheds public-key; sat for members, nil for owners.
type Database struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	CreatedAt        string  `json:"created_at"`
	Role             string  `json:"role,omitempty"`
	WrappedMasterKey *string `json:"wrapped_master_key,omitempty"`
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

// UserLookup er svaret fra GET /users/lookup. TargetDevice er den enhed
// vi skal wrappe master_key til (nyeste enhed med public_key for den
// fundne bruger).
type UserLookup struct {
	User struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
	TargetDevice struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		PublicKey  string `json:"public_key"`
		EnrolledAt string `json:"enrolled_at"`
	} `json:"target_device"`
}

// LookupUser slår en bruger op pa username og returnerer info om den
// "target device" Alice's klient skal wrappe master_key til.
func (c *Client) LookupUser(ctx context.Context, deviceToken, username string) (*UserLookup, error) {
	u := c.baseURL + "/api/v1/users/lookup?username=" + url.QueryEscape(username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out UserLookup
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// ShareMember er én række i list-shares respons.
type ShareMember struct {
	UserID      string  `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
	Role        string  `json:"role"`
	AddedAt     string  `json:"added_at"`
	AddedBy     *string `json:"added_by"`
}

// ListShares henter medlems-listen for en database (owner-only på server).
func (c *Client) ListShares(ctx context.Context, deviceToken, databaseID string) ([]ShareMember, error) {
	u := fmt.Sprintf("%s/api/v1/databases/%s/shares", c.baseURL, databaseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out struct {
		Members []ShareMember `json:"members"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.Members, nil
}

// ShareDatabase tilfojer (eller roterer) en members wrapped_master_key.
func (c *Client) ShareDatabase(ctx context.Context, deviceToken, databaseID, targetUserID string, wrappedKey []byte) error {
	buf, err := json.Marshal(map[string]string{
		"user_id":            targetUserID,
		"wrapped_master_key": base64.StdEncoding.EncodeToString(wrappedKey),
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	u := fmt.Sprintf("%s/api/v1/databases/%s/shares", c.baseURL, databaseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("share database: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return parseError(resp)
	}
	return nil
}

// UnshareDatabase fjerner en member (eller lader member forlade selv).
func (c *Client) UnshareDatabase(ctx context.Context, deviceToken, databaseID, targetUserID string) error {
	u := fmt.Sprintf("%s/api/v1/databases/%s/shares/%s", c.baseURL, databaseID, targetUserID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("unshare database: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return parseError(resp)
	}
	return nil
}

// EntryChange er én række i GET /changes-svaret: nyeste version af en entry
// med wire-format blob (base64-encoded nonce ‖ ciphertext).
type EntryChange struct {
	UUID              string `json:"uuid"`
	Blob              string `json:"blob"`
	ModifiedAt        string `json:"modified_at"`
	Deleted           bool   `json:"deleted"`
	Seq               int64  `json:"seq"`
	AvailableVersions int    `json:"available_versions"`
}

// ChangesResponse er svaret fra GET /databases/{id}/changes.
type ChangesResponse struct {
	CurrentSeq int64         `json:"current_seq"`
	Entries    []EntryChange `json:"entries"`
}

// GetChanges henter alle entry-versioner med server_seq > since. Serveren
// returnerer kun nyeste version pr. entry; ældre versioner kræver separate
// GetVersions/GetVersion-kald.
func (c *Client) GetChanges(ctx context.Context, deviceToken, databaseID string, since int64) (*ChangesResponse, error) {
	u := fmt.Sprintf("%s/api/v1/databases/%s/changes?since=%d", c.baseURL, databaseID, since)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get changes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out ChangesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// EntryPutResponse er det server-svar PUT/DELETE returnerer for en entry.
type EntryPutResponse struct {
	UUID       string `json:"uuid"`
	ModifiedAt string `json:"modified_at"`
	Deleted    bool   `json:"deleted"`
	Seq        int64  `json:"seq"`
	CreatedAt  string `json:"created_at"`
}

// PutEntry uploader en ny entry-version. blob er den rå wire-format
// (nonce ‖ ciphertext); klienten base64-encoder undervejs. modifiedAt er den
// KeePass-side last-modified timestamp og må gerne være i fortiden.
func (c *Client) PutEntry(ctx context.Context, deviceToken, databaseID, entryUUID string, blob []byte, modifiedAt time.Time) (*EntryPutResponse, error) {
	return c.writeEntry(ctx, http.MethodPut, deviceToken, databaseID, entryUUID, blob, modifiedAt, true)
}

// DeleteEntry markerer entry'en som tombstone. Tæller som en ny version på
// serveren. blob er typisk nil/tom (server accepterer manglende blob).
func (c *Client) DeleteEntry(ctx context.Context, deviceToken, databaseID, entryUUID string, blob []byte, modifiedAt time.Time) (*EntryPutResponse, error) {
	return c.writeEntry(ctx, http.MethodDelete, deviceToken, databaseID, entryUUID, blob, modifiedAt, false)
}

func (c *Client) writeEntry(ctx context.Context, method, deviceToken, databaseID, entryUUID string, blob []byte, modifiedAt time.Time, blobRequired bool) (*EntryPutResponse, error) {
	body := map[string]any{
		"modified_at": modifiedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if blob != nil {
		body["blob"] = base64.StdEncoding.EncodeToString(blob)
	} else if blobRequired {
		return nil, errors.New("blob is required for PUT")
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	u := fmt.Sprintf("%s/api/v1/databases/%s/entries/%s", c.baseURL, databaseID, entryUUID)
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s entry: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var out struct {
		Entry EntryPutResponse `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out.Entry, nil
}

// EntryVersion er én historisk version af en entry.
type EntryVersion struct {
	VersionNum int    `json:"version_num"`
	ModifiedAt string `json:"modified_at"`
	CreatedAt  string `json:"created_at"`
	Deleted    bool   `json:"deleted"`
	Blob       string `json:"blob"`
}

// GetVersions lister alle bevarede versioner (op til 3) af en entry, nyeste først.
func (c *Client) GetVersions(ctx context.Context, deviceToken, databaseID, entryUUID string) ([]EntryVersion, error) {
	u := fmt.Sprintf("%s/api/v1/databases/%s/entries/%s/versions", c.baseURL, databaseID, entryUUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out struct {
		EntryUUID string         `json:"entry_uuid"`
		Versions  []EntryVersion `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.Versions, nil
}

// GetVersion henter én specifik version (num 1, 2 eller 3) af en entry.
func (c *Client) GetVersion(ctx context.Context, deviceToken, databaseID, entryUUID string, num int) (*EntryVersion, error) {
	u := fmt.Sprintf("%s/api/v1/databases/%s/entries/%s/versions/%d", c.baseURL, databaseID, entryUUID, num)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out EntryVersion
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// RestoreVersion ruller en gammel version frem som ny nyeste. Serveren bumper
// server_seq og bevarer historik (modsat overwrite).
type RestoreResponse struct {
	UUID          string `json:"uuid"`
	ModifiedAt    string `json:"modified_at"`
	Deleted       bool   `json:"deleted"`
	Seq           int64  `json:"seq"`
	CreatedAt     string `json:"created_at"`
	RestoredFrom  int    `json:"restored_from"`
}

func (c *Client) RestoreVersion(ctx context.Context, deviceToken, databaseID, entryUUID string, num int) (*RestoreResponse, error) {
	u := fmt.Sprintf("%s/api/v1/databases/%s/entries/%s/restore/%d", c.baseURL, databaseID, entryUUID, num)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}
	c.authJSON(req, deviceToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("restore version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out struct {
		Entry RestoreResponse `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out.Entry, nil
}

// authJSON sætter standard-headers for autentificerede JSON-requests.
func (c *Client) authJSON(req *http.Request, deviceToken string) {
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
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
