// Package authclient reads the restaurant list from franchise-service, which
// owns restaurants (franchises). Used to seed a starter menu per restaurant on
// first boot.
package authclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// restaurantsPath is franchise-service's public (anonymous) restaurant list,
// active restaurants only.
const restaurantsPath = "/api/restaurants"

// Tenant is a restaurant as exposed by franchise-service.
type Tenant struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	IsActive bool   `json:"isActive"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a client for the franchise-service root URL (e.g.
// http://franchise-service:8089); the restaurants path is appended per call.
func New(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) ListTenants(ctx context.Context) ([]Tenant, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+restaurantsPath, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("franchise-service returned %d", resp.StatusCode)
	}
	var restaurants []Tenant
	if err := json.NewDecoder(resp.Body).Decode(&restaurants); err != nil {
		return nil, err
	}
	return restaurants, nil
}
