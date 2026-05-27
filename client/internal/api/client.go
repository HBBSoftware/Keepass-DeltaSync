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
