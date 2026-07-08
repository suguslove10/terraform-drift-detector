package provider

import (
	"context"

	"terraform-drift-detector/internal/models"
)

// Provider defines the interface that all cloud providers must implement.
// It fetches the actual (live) state of a resource from the cloud.
type Provider interface {
	// Name returns the provider name (e.g., "aws", "mock").
	Name() string

	// FetchActual retrieves the current state of a resource from the cloud.
	// Returns nil if the resource has been deleted or does not exist.
	// The expected attributes and tags are passed so the provider knows which fields to fetch.
	FetchActual(ctx context.Context, resourceType, resourceID string, expectedAttrs map[string]any, expectedTags map[string]string) (*models.ResourceState, error)

	// SupportedTypes returns the list of resource types this provider can handle.
	SupportedTypes() []string
}
