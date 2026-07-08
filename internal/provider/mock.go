package provider

import (
	"context"
	"fmt"
	"strings"

	"terraform-drift-detector/internal/models"
)

// MockProvider simulates cloud infrastructure for testing and demos.
// It uses naming conventions to deterministically simulate drift:
//   - Names containing "deleted" -> resource not found (DELETED)
//   - Names containing "drifted" -> attributes modified (DRIFTED)
//   - Otherwise -> returns matching state (IN_SYNC)
type MockProvider struct{}

func init() {
	Register("mock", &MockProvider{})
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) SupportedTypes() []string {
	return []string{
		"mock_instance",
		"mock_s3_bucket",
		"mock_security_group",
		"mock_database",
	}
}

func (m *MockProvider) FetchActual(ctx context.Context, resourceType, resourceID string, expectedAttrs map[string]any, expectedTags map[string]string) (*models.ResourceState, error) {
	// Simulate deleted resources
	if strings.Contains(strings.ToLower(resourceID), "deleted") || strings.Contains(strings.ToLower(resourceID), "_deleted") {
		return nil, nil
	}

	// Build base actual state matching expected
	actual := &models.ResourceState{
		ID:         resourceID,
		Type:       resourceType,
		Provider:   "mock",
		Attributes: make(map[string]any),
		Tags:       make(map[string]string),
	}

	// Copy expected attributes to actual
	for k, v := range expectedAttrs {
		actual.Attributes[k] = v
	}

	// Copy expected tags to actual
	for k, v := range expectedTags {
		actual.Tags[k] = v
	}

	// Simulate drifted resources
	if strings.Contains(strings.ToLower(resourceID), "drifted") || strings.Contains(strings.ToLower(resourceID), "_drifted") {
		m.applyDrift(resourceType, actual)
	}

	return actual, nil
}

// applyDrift modifies the actual state to simulate configuration drift.
func (m *MockProvider) applyDrift(resourceType string, actual *models.ResourceState) {
	switch resourceType {
	case "mock_instance":
		// Change instance type
		if _, ok := actual.Attributes["instance_type"]; ok {
			actual.Attributes["instance_type"] = "t3.xlarge"
		}
		// Change a security group
		if _, ok := actual.Attributes["vpc_security_group_ids"]; ok {
			actual.Attributes["vpc_security_group_ids"] = []any{"sg-changed111", "sg-changed222"}
		}
		// Modify tags
		actual.Tags["Environment"] = "staging"
		actual.Tags["DriftedTag"] = "unexpected-value"

	case "mock_s3_bucket":
		// Change versioning
		if _, ok := actual.Attributes["versioning_enabled"]; ok {
			actual.Attributes["versioning_enabled"] = false
		}
		// Change ACL
		if _, ok := actual.Attributes["acl"]; ok {
			actual.Attributes["acl"] = "public-read"
		}
		// Modify tags
		actual.Tags["Environment"] = "development"
		delete(actual.Tags, "ManagedBy")
		actual.Tags["UnexpectedTag"] = "added-manually"

	case "mock_security_group":
		// Change ingress rule
		if _, ok := actual.Attributes["ingress_cidr"]; ok {
			actual.Attributes["ingress_cidr"] = "0.0.0.0/0"
		}
		// Change ingress port
		if _, ok := actual.Attributes["ingress_from_port"]; ok {
			actual.Attributes["ingress_from_port"] = float64(0)
		}
		// Modify description
		if _, ok := actual.Attributes["description"]; ok {
			actual.Attributes["description"] = "Modified manually via console"
		}
		actual.Tags["Environment"] = "unknown"

	case "mock_database":
		// Change instance class
		if _, ok := actual.Attributes["instance_class"]; ok {
			actual.Attributes["instance_class"] = "db.r5.2xlarge"
		}
		// Change storage
		if _, ok := actual.Attributes["allocated_storage"]; ok {
			actual.Attributes["allocated_storage"] = float64(200)
		}
		// Change backup retention
		if _, ok := actual.Attributes["backup_retention_period"]; ok {
			actual.Attributes["backup_retention_period"] = float64(1)
		}
		actual.Tags["Environment"] = "disaster-recovery"

	default:
		// Generic drift: modify first string attribute found
		for k, v := range actual.Attributes {
			if s, ok := v.(string); ok {
				actual.Attributes[k] = fmt.Sprintf("%s-modified", s)
				break
			}
		}
		actual.Tags["DriftedTag"] = "unexpected"
	}
}
