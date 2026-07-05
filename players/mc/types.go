package mc

import "encoding/json"

type Profile struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type StatusResponse struct {
	Players struct {
		Online int       `json:"online"`
		Max    int       `json:"max"`
		Sample []Profile `json:"sample"`
	} `json:"players"`
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Description json.RawMessage `json:"description"`
}
