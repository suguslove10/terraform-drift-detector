package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStateFile(t *testing.T) {
	// Create a temp state file
	stateJSON := `{
		"version": 4,
		"terraform_version": "1.5.7",
		"serial": 1,
		"lineage": "test-lineage",
		"resources": [
			{
				"mode": "managed",
				"type": "mock_instance",
				"name": "test_server",
				"provider": "provider[\"registry.terraform.io/hashicorp/mock\"]",
				"instances": [
					{
						"schema_version": 0,
						"attributes": {
							"id": "i-test001",
							"instance_type": "t3.medium",
							"ami": "ami-12345",
							"tags": {
								"Name": "test-server",
								"Environment": "production"
							}
						}
					}
				]
			},
			{
				"mode": "data",
				"type": "aws_ami",
				"name": "ubuntu",
				"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
				"instances": [
					{
						"schema_version": 0,
						"attributes": {
							"id": "ami-99999"
						}
					}
				]
			}
		]
	}`

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.tfstate")
	if err := os.WriteFile(statePath, []byte(stateJSON), 0644); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	resources, err := ParseStateFile(statePath)
	if err != nil {
		t.Fatalf("ParseStateFile() error: %v", err)
	}

	// Should only include managed resources (not data sources)
	if len(resources) != 1 {
		t.Fatalf("expected 1 managed resource, got %d", len(resources))
	}

	r := resources[0]

	if r.ID != "i-test001" {
		t.Errorf("expected ID 'i-test001', got %q", r.ID)
	}

	if r.Type != "mock_instance" {
		t.Errorf("expected Type 'mock_instance', got %q", r.Type)
	}

	if r.Name != "test_server" {
		t.Errorf("expected Name 'test_server', got %q", r.Name)
	}

	if r.Provider != "mock" {
		t.Errorf("expected Provider 'mock', got %q", r.Provider)
	}

	// Check tags extracted correctly
	if r.Tags["Name"] != "test-server" {
		t.Errorf("expected tag Name='test-server', got %q", r.Tags["Name"])
	}

	if r.Tags["Environment"] != "production" {
		t.Errorf("expected tag Environment='production', got %q", r.Tags["Environment"])
	}

	// Check that tracked attributes don't include id or tags
	if _, ok := r.Attributes["id"]; ok {
		t.Error("attributes should not contain 'id'")
	}
	if _, ok := r.Attributes["tags"]; ok {
		t.Error("attributes should not contain 'tags'")
	}

	// Check actual attributes
	if r.Attributes["instance_type"] != "t3.medium" {
		t.Errorf("expected instance_type='t3.medium', got %v", r.Attributes["instance_type"])
	}
}

func TestParseStateFileUnsupportedVersion(t *testing.T) {
	stateJSON := `{"version": 3, "resources": []}`

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "old.tfstate")
	os.WriteFile(statePath, []byte(stateJSON), 0644)

	_, err := ParseStateFile(statePath)
	if err == nil {
		t.Fatal("expected error for unsupported version, got nil")
	}
}

func TestExtractProviderName(t *testing.T) {
	tests := []struct {
		providerPath string
		resourceType string
		expected     string
	}{
		{`provider["registry.terraform.io/hashicorp/aws"]`, "aws_instance", "aws"},
		{`provider["registry.terraform.io/hashicorp/google"]`, "google_compute_instance", "google"},
		{"", "aws_s3_bucket", "aws"},
		{"", "mock_instance", "mock"},
	}

	for _, tt := range tests {
		got := extractProviderName(tt.providerPath, tt.resourceType)
		if got != tt.expected {
			t.Errorf("extractProviderName(%q, %q) = %q, want %q",
				tt.providerPath, tt.resourceType, got, tt.expected)
		}
	}
}
