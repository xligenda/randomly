package mc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type identifierType int

const (
	byUUID identifierType = iota
	byUsername
)

// FetchProfile retrieves a Minecraft player's profile using either their UUID or Username.
func FetchProfile(identifier string, timeout time.Duration) (*Profile, error) {
	idType := byUsername
	if len(identifier) == 32 || len(identifier) == 36 { // 36 accounts for dashed UUIDs
		idType = byUUID
	}

	var url string
	switch idType {
	case byUUID:
		url = fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/profile/%s", identifier)
	case byUsername:
		url = fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", identifier)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("player with identifier %q not found", identifier)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from mojang server", resp.StatusCode)
	}

	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &profile, nil
}
