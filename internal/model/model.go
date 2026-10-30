package model

import "time"

type Container struct {
	Project string    `json:"project"`
	Service string    `json:"service"`
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Image   string    `json:"image"`
	State   string    `json:"state"`
	SeenAt  time.Time `json:"seen_at"`
	Present bool      `json:"present"`
	Latest  *Sample   `json:"latest"`
}

// Event stores only selected lifecycle metadata, never arbitrary labels or logs.
type Event struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	Project     string    `json:"project"`
	Service     string    `json:"service"`
	Action      string    `json:"action"`
	At          time.Time `json:"at"`
	ExitCode    string    `json:"exit_code,omitempty"`
	Signal      string    `json:"signal,omitempty"`
}

type Sample struct {
	ContainerID    string    `json:"container_id"`
	At             time.Time `json:"at"`
	CPU            *float64  `json:"cpu_percent"`
	Memory         int64     `json:"memory_bytes"`
	MemoryLimit    int64     `json:"memory_limit_bytes"`
	NetworkRx      *int64    `json:"network_rx_bytes"`
	NetworkTx      *int64    `json:"network_tx_bytes"`
	BlockRead      *int64    `json:"block_read_bytes"`
	BlockWrite     *int64    `json:"block_write_bytes"`
	PIDs           *int64    `json:"pids"`
	NetworkRxRate  *float64  `json:"network_rx_bytes_per_second"`
	NetworkTxRate  *float64  `json:"network_tx_bytes_per_second"`
	BlockReadRate  *float64  `json:"block_read_bytes_per_second"`
	BlockWriteRate *float64  `json:"block_write_bytes_per_second"`
}
