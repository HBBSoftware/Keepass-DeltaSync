// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// AdminUser er én række fra GET /admin/users.
type AdminUser struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	DisplayName   *string `json:"display_name"`
	CreatedAt     string  `json:"created_at"`
	Disabled      bool    `json:"disabled"`
	DeviceCount   int     `json:"device_count"`
	DatabaseCount int     `json:"database_count"`
}

// AdminCreateUserResponse er server-svaret ved POST /admin/users — inkluderer
// både den oprettede bruger og en engangs-enrollment-token til første enhed.
type AdminCreateUserResponse struct {
	User struct {
		ID          string  `json:"id"`
		Username    string  `json:"username"`
		DisplayName *string `json:"display_name"`
		CreatedAt   string  `json:"created_at"`
	} `json:"user"`
	EnrollmentToken string `json:"enrollment_token"`
	ExpiresAt       string `json:"expires_at"`
}

// AdminListUsers henter alle brugere via GET /api/v1/admin/users.
func (c *Client) AdminListUsers(ctx context.Context, adminToken string) ([]AdminUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/admin/users", nil)
	if err != nil {
		return nil, err
	}
	c.authJSON(req, adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("admin list users: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out struct {
		Users []AdminUser `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.Users, nil
}

// AdminCreateUser opretter en ny bruger via POST /api/v1/admin/users.
// displayName er valgfri (tom = nil i body).
func (c *Client) AdminCreateUser(ctx context.Context, adminToken, username, displayName string) (*AdminCreateUserResponse, error) {
	body := map[string]any{"username": username}
	if displayName != "" {
		body["display_name"] = displayName
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/admin/users", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	c.authJSON(req, adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("admin create user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp)
	}
	var out AdminCreateUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.EnrollmentToken == "" {
		return nil, errors.New("server returned empty enrollment_token")
	}
	return &out, nil
}

// AdminSetUserDisabled sætter disabled-flag på en bruger via PATCH
// /api/v1/admin/users/{id}. userID er UUID — caller'en skal slå username
// op først via AdminListUsers.
func (c *Client) AdminSetUserDisabled(ctx context.Context, adminToken, userID string, disabled bool) error {
	buf, err := json.Marshal(map[string]bool{"disabled": disabled})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	u := fmt.Sprintf("%s/api/v1/admin/users/%s", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	c.authJSON(req, adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("admin set disabled: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}
	return nil
}

// AdminDeleteUser sletter en bruger permanent via DELETE /api/v1/admin/users/{id}.
// CASCADE fjerner devices, databaser, entries.
func (c *Client) AdminDeleteUser(ctx context.Context, adminToken, userID string) error {
	u := fmt.Sprintf("%s/api/v1/admin/users/%s", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	c.authJSON(req, adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("admin delete user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}
	return nil
}

// AdminCreateEnrollmentToken genererer en ny engangs-enrollment-token til
// en eksisterende bruger via POST /api/v1/admin/users/{id}/enrollment.
// Bruges fx når brugeren skal enrolle en ekstra enhed.
func (c *Client) AdminCreateEnrollmentToken(ctx context.Context, adminToken, userID string) (token, expiresAt string, err error) {
	u := fmt.Sprintf("%s/api/v1/admin/users/%s/enrollment", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return "", "", err
	}
	c.authJSON(req, adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("admin create enrollment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", parseError(resp)
	}
	var out struct {
		EnrollmentToken string `json:"enrollment_token"`
		ExpiresAt       string `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.EnrollmentToken == "" {
		return "", "", errors.New("server returned empty enrollment_token")
	}
	return out.EnrollmentToken, out.ExpiresAt, nil
}
