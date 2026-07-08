package parser

import (
	"encoding/json"
	"fmt"
	"os"

	"terraform-drift-detector/internal/models"
)

// TFState represents the top-level Terraform state file structure (v4).
type TFState struct {
	Version          int          `json:"version"`
	TerraformVersion string       `json:"terraform_version"`
	Serial           int          `json:"serial"`
	Lineage          string       `json:"lineage"`
	Resources        []TFResource `json:"resources"`
}

// TFResource represents a single resource block in Terraform state.
type TFResource struct {
	Mode      string       `json:"mode"`
	Type      string       `json:"type"`
	Name      string       `json:"name"`
	Provider  string       `json:"provider"`
	Instances []TFInstance `json:"instances"`
}

// TFInstance represents a resource instance in Terraform state.
type TFInstance struct {
	SchemaVersion int            `json:"schema_version"`
	Attributes    map[string]any `json:"attributes"`
}

// ParseStateFile reads a Terraform .tfstate file and extracts managed resources
// as normalized ResourceState objects.
func ParseStateFile(filePath string) ([]models.ResourceState, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file %s: %w", filePath, err)
	}

	var state TFState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file %s: %w", filePath, err)
	}

	if state.Version != 4 {
		return nil, fmt.Errorf("unsupported state file version: %d (expected 4)", state.Version)
	}

	var resources []models.ResourceState

	for _, res := range state.Resources {
		// Only process managed resources (skip data sources)
		if res.Mode != "managed" {
			continue
		}

		if len(res.Instances) == 0 {
			continue
		}

		// Use first instance (index 0) — handles non-count/for_each resources
		attrs := res.Instances[0].Attributes
		if attrs == nil {
			attrs = make(map[string]any)
		}

		// Extract resource ID
		id, _ := attrs["id"].(string)

		// Extract and normalize tags
		tags := extractTags(attrs)

		// Extract provider name from the provider path
		providerName := extractProviderName(res.Provider, res.Type)

		// Build tracked attributes (excluding id, tags, and internal fields)
		trackedAttrs := make(map[string]any)
		for k, v := range attrs {
			if k == "id" || k == "tags" || k == "tags_all" {
				continue
			}
			trackedAttrs[k] = v
		}

		resources = append(resources, models.ResourceState{
			ID:         id,
			Type:       res.Type,
			Name:       res.Name,
			Provider:   providerName,
			Attributes: trackedAttrs,
			Tags:       tags,
		})
	}

	return resources, nil
}

// extractTags extracts the tags map from resource attributes.
func extractTags(attrs map[string]any) map[string]string {
	tags := make(map[string]string)

	rawTags, ok := attrs["tags"]
	if !ok {
		return tags
	}

	switch t := rawTags.(type) {
	case map[string]any:
		for k, v := range t {
			if s, ok := v.(string); ok {
				tags[k] = s
			} else {
				tags[k] = fmt.Sprintf("%v", v)
			}
		}
	case map[string]string:
		return t
	}

	return tags
}

// extractProviderName extracts a short provider name from the Terraform provider path
// or infers it from the resource type prefix.
// e.g., "registry.terraform.io/hashicorp/aws" -> "aws"
// e.g., resource type "aws_instance" -> "aws"
func extractProviderName(providerPath, resourceType string) string {
	// Try to extract from provider path (format: registry.terraform.io/hashicorp/<provider>)
	if providerPath != "" {
		// Find last segment
		for i := len(providerPath) - 1; i >= 0; i-- {
			if providerPath[i] == '/' {
				name := providerPath[i+1:]
				// Remove any surrounding quotes or brackets
				name = cleanProviderName(name)
				if name != "" {
					return name
				}
				break
			}
		}
	}

	// Fallback: infer from resource type prefix (e.g., "aws_instance" -> "aws")
	for i, c := range resourceType {
		if c == '_' {
			return resourceType[:i]
		}
	}

	return "unknown"
}

// cleanProviderName removes surrounding quotes and brackets from provider name.
func cleanProviderName(name string) string {
	result := make([]byte, 0, len(name))
	for _, c := range name {
		if c != '"' && c != '\'' && c != '[' && c != ']' {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
