package provision

import "time"

type Device struct {
	Serial  string `json:"serial"`
	Product string `json:"product"`
	Path    string `json:"path"`
}
type Capabilities struct {
	CLI        bool   `json:"cli"`
	Golden     bool   `json:"golden"`
	Digest     bool   `json:"digest"`
	Blocks     bool   `json:"blocks"`
	BoardFirst bool   `json:"boardFirst"`
	Version    string `json:"version"`
	Problem    string `json:"problem"`
}
type Status struct {
	Smoke        bool         `json:"smoke"`
	Demo         bool         `json:"demo"`
	Running      bool         `json:"running"`
	Capabilities Capabilities `json:"capabilities"`
	Logs         string       `json:"logs"`
	Job          string       `json:"job"`
}
type Request struct {
	Mode           string   `json:"mode"`
	Board          string   `json:"board"`
	Image          string   `json:"image"`
	Source         string   `json:"source"`
	Targets        []string `json:"targets"`
	Programmer     string   `json:"programmer"`
	Version        string   `json:"version"`
	OS             string   `json:"os"`
	Strategy       string   `json:"strategy"`
	Jobs           int      `json:"jobs"`
	TimeoutMinutes int      `json:"timeoutMinutes"`
	Ready          bool     `json:"ready"`
	Confirmed      bool     `json:"confirmed"`
}
type Plan struct {
	Title        string     `json:"title"`
	Explanation  string     `json:"explanation"`
	Verification string     `json:"verification"`
	Targets      []string   `json:"targets"`
	Excluded     string     `json:"excluded"`
	Jobs         int        `json:"jobs"`
	Commands     [][]string `json:"commands"`
	Warnings     []string   `json:"warnings"`
	Bytes        uint64     `json:"bytes"`
}
type ImageInfo struct {
	Path   string `json:"path"`
	Board  string `json:"board"`
	Source string `json:"source"`
	Bytes  uint64 `json:"bytes"`
	LUNs   int    `json:"luns"`
	Name   string `json:"name"`
}
type Event struct {
	Job     string    `json:"job"`
	Kind    string    `json:"kind"`
	Serial  string    `json:"serial"`
	Phase   string    `json:"phase"`
	Percent float64   `json:"percent"`
	Rate    string    `json:"rate"`
	ETA     string    `json:"eta"`
	Elapsed string    `json:"elapsed"`
	Message string    `json:"message"`
	Status  string    `json:"status"`
	Time    time.Time `json:"time"`
}
type Result struct {
	Serial       string  `json:"serial"`
	Status       string  `json:"status"`
	Seconds      float64 `json:"seconds"`
	Error        string  `json:"error,omitempty"`
	BytesWritten uint64  `json:"bytes_written,omitempty"`
	BytesSkipped uint64  `json:"bytes_skipped,omitempty"`
}
