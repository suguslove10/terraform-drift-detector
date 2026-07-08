package models

import (
	"time"
)

// ResourceState represents a normalized cloud resource, whether from Terraform state or actual cloud.
type ResourceState struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Provider   string            `json:"provider"`
	Attributes map[string]any    `json:"attributes"`
	Tags       map[string]string `json:"tags"`
}

// AttributeDiff represents a single attribute difference between expected and actual state.
type AttributeDiff struct {
	Name     string `json:"name"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
}

// TagDiff represents a single tag difference.
type TagDiff struct {
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Status   string `json:"status"` // "modified", "added", "removed"
}

// DriftStatus constants.
const (
	StatusInSync  = "IN_SYNC"
	StatusDrifted = "DRIFTED"
	StatusDeleted = "DELETED"
)

// ResourceDrift represents drift information for a single resource.
type ResourceDrift struct {
	ResourceID     string             `json:"resource_id"`
	Type           string             `json:"type"`
	Name           string             `json:"name"`
	Status         string             `json:"status"`
	AttributeDiffs []AttributeDiff    `json:"attribute_diffs,omitempty"`
	TagDiffs       map[string]TagDiff `json:"tag_diffs,omitempty"`
}

// DriftReport is the full report of a drift scan.
type DriftReport struct {
	ID             string          `json:"id"`
	Timestamp      string          `json:"timestamp"`
	StateFile      string          `json:"state_file"`
	Provider       string          `json:"provider"`
	TotalResources int             `json:"total_resources"`
	InSyncCount    int             `json:"in_sync_count"`
	DriftedCount   int             `json:"drifted_count"`
	DeletedCount   int             `json:"deleted_count"`
	Drifts         []ResourceDrift `json:"drifts"`
}

// NewDriftReport creates a new DriftReport with a generated ID and timestamp.
func NewDriftReport(stateFile, provider string) *DriftReport {
	now := time.Now()
	return &DriftReport{
		ID:        now.Format("20060102-150405"),
		Timestamp: now.Format(time.RFC3339),
		StateFile: stateFile,
		Provider:  provider,
		Drifts:    make([]ResourceDrift, 0),
	}
}

// ScheduleConfig holds schedule configuration for recurring scans.
type ScheduleConfig struct {
	ID        string `json:"id"`
	StateFile string `json:"state_file"`
	Provider  string `json:"provider"`
	Interval  string `json:"interval"` // e.g. "5m", "1h", "30s"
	Enabled   bool   `json:"enabled"`
	LastRun   string `json:"last_run,omitempty"`
}
