package comparator

import (
	"context"
	"testing"

	"terraform-drift-detector/internal/models"
	_ "terraform-drift-detector/internal/provider"
)

func TestCompareWithMockProvider(t *testing.T) {
	resources := []models.ResourceState{
		{
			ID:       "i-web001",
			Type:     "mock_instance",
			Name:     "web_server",
			Provider: "mock",
			Attributes: map[string]any{
				"instance_type": "t3.medium",
				"ami":           "ami-12345",
			},
			Tags: map[string]string{
				"Name":        "web-server",
				"Environment": "production",
			},
		},
		{
			ID:       "i-api-drifted-002",
			Type:     "mock_instance",
			Name:     "api_server_drifted",
			Provider: "mock",
			Attributes: map[string]any{
				"instance_type":          "t3.medium",
				"vpc_security_group_ids": []any{"sg-abc123"},
			},
			Tags: map[string]string{
				"Environment": "production",
			},
		},
		{
			ID:       "i-old-deleted-003",
			Type:     "mock_instance",
			Name:     "old_server_deleted",
			Provider: "mock",
			Attributes: map[string]any{
				"instance_type": "t2.micro",
			},
			Tags: map[string]string{},
		},
	}

	report, err := Compare(context.Background(), resources, "mock", "test.tfstate")
	if err != nil {
		t.Fatalf("Compare() error: %v", err)
	}

	if report.TotalResources != 3 {
		t.Errorf("expected 3 total resources, got %d", report.TotalResources)
	}

	// Count statuses
	statusCounts := map[string]int{}
	for _, drift := range report.Drifts {
		statusCounts[drift.Status]++
	}

	if statusCounts[models.StatusInSync] != 1 {
		t.Errorf("expected 1 IN_SYNC, got %d", statusCounts[models.StatusInSync])
	}

	if statusCounts[models.StatusDrifted] != 1 {
		t.Errorf("expected 1 DRIFTED, got %d", statusCounts[models.StatusDrifted])
	}

	if statusCounts[models.StatusDeleted] != 1 {
		t.Errorf("expected 1 DELETED, got %d", statusCounts[models.StatusDeleted])
	}
}

func TestCompareAttributes(t *testing.T) {
	expected := map[string]any{
		"instance_type": "t3.medium",
		"ami":           "ami-12345",
		"monitoring":    true,
	}

	actual := map[string]any{
		"instance_type": "t3.xlarge", // changed
		"ami":           "ami-12345", // same
	}

	diffs := compareAttributes(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %+v", len(diffs), diffs)
	}

	diffMap := make(map[string]models.AttributeDiff)
	for _, d := range diffs {
		diffMap[d.Name] = d
	}

	if d, ok := diffMap["instance_type"]; ok {
		if d.Expected != "t3.medium" || d.Actual != "t3.xlarge" {
			t.Errorf("instance_type diff unexpected: %+v", d)
		}
	} else {
		t.Error("missing diff for instance_type")
	}
}

func TestCompareTags(t *testing.T) {
	expected := map[string]string{
		"Name":        "web-server",
		"Environment": "production",
		"ManagedBy":   "terraform",
	}

	actual := map[string]string{
		"Name":        "web-server",     // same
		"Environment": "staging",        // modified
		"NewTag":      "added-manually", // added
		// ManagedBy removed
	}

	diffs := compareTags(expected, actual)

	if len(diffs) != 3 {
		t.Fatalf("expected 3 tag diffs, got %d", len(diffs))
	}

	if d, ok := diffs["Environment"]; ok {
		if d.Status != "modified" {
			t.Errorf("expected Environment status 'modified', got %q", d.Status)
		}
	} else {
		t.Error("missing tag diff for Environment")
	}

	if d, ok := diffs["ManagedBy"]; ok {
		if d.Status != "removed" {
			t.Errorf("expected ManagedBy status 'removed', got %q", d.Status)
		}
	} else {
		t.Error("missing tag diff for ManagedBy")
	}

	if d, ok := diffs["NewTag"]; ok {
		if d.Status != "added" {
			t.Errorf("expected NewTag status 'added', got %q", d.Status)
		}
	} else {
		t.Error("missing tag diff for NewTag")
	}
}
