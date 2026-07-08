package comparator

import (
	"context"
	"fmt"
	"sync"

	"terraform-drift-detector/internal/models"
	"terraform-drift-detector/internal/provider"
)

// MaxConcurrency is the maximum number of parallel cloud API calls.
const MaxConcurrency = 10

// Compare takes a list of expected resources (from state) and a provider name,
// fetches the actual state for each resource in parallel, and returns a DriftReport.
func Compare(ctx context.Context, resources []models.ResourceState, providerName, stateFile string) (*models.DriftReport, error) {
	p, err := provider.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	report := models.NewDriftReport(stateFile, providerName)
	report.TotalResources = len(resources)

	// Channel-based semaphore for concurrency control
	sem := make(chan struct{}, MaxConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, res := range resources {
		wg.Add(1)
		go func(expected models.ResourceState) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			drift := compareResource(ctx, p, expected)

			mu.Lock()
			defer mu.Unlock()

			report.Drifts = append(report.Drifts, drift)
			switch drift.Status {
			case models.StatusInSync:
				report.InSyncCount++
			case models.StatusDrifted:
				report.DriftedCount++
			case models.StatusDeleted:
				report.DeletedCount++
			}
		}(res)
	}

	wg.Wait()
	return report, nil
}

// compareResource fetches the actual state for a single resource and computes its drift.
func compareResource(ctx context.Context, p provider.Provider, expected models.ResourceState) models.ResourceDrift {
	drift := models.ResourceDrift{
		ResourceID: expected.ID,
		Type:       expected.Type,
		Name:       expected.Name,
	}

	actual, err := p.FetchActual(ctx, expected.Type, expected.ID, expected.Attributes, expected.Tags)
	if err != nil {
		// Treat provider errors as drift with a note
		drift.Status = models.StatusDrifted
		drift.AttributeDiffs = []models.AttributeDiff{
			{Name: "_error", Expected: "accessible", Actual: err.Error()},
		}
		return drift
	}

	// Resource deleted
	if actual == nil {
		drift.Status = models.StatusDeleted
		return drift
	}

	// Compare attributes
	attrDiffs := compareAttributes(expected.Attributes, actual.Attributes)

	// Compare tags
	tagDiffs := compareTags(expected.Tags, actual.Tags)

	if len(attrDiffs) == 0 && len(tagDiffs) == 0 {
		drift.Status = models.StatusInSync
	} else {
		drift.Status = models.StatusDrifted
		drift.AttributeDiffs = attrDiffs
		drift.TagDiffs = tagDiffs
	}

	return drift
}

// compareAttributes compares expected vs actual attributes and returns differences.
func compareAttributes(expected, actual map[string]any) []models.AttributeDiff {
	var diffs []models.AttributeDiff

	for key, actualVal := range actual {
		expectedVal, exists := expected[key]
		if !exists {
			// Skip actual attributes that are not defined in expected state
			continue
		}

		if !deepEqual(expectedVal, actualVal) {
			diffs = append(diffs, models.AttributeDiff{
				Name:     key,
				Expected: expectedVal,
				Actual:   actualVal,
			})
		}
	}

	return diffs
}

// compareTags compares expected vs actual tags and returns differences.
func compareTags(expected, actual map[string]string) map[string]models.TagDiff {
	diffs := make(map[string]models.TagDiff)

	for key, expectedVal := range expected {
		actualVal, exists := actual[key]
		if !exists {
			diffs[key] = models.TagDiff{
				Expected: expectedVal,
				Actual:   "",
				Status:   "removed",
			}
		} else if expectedVal != actualVal {
			diffs[key] = models.TagDiff{
				Expected: expectedVal,
				Actual:   actualVal,
				Status:   "modified",
			}
		}
	}

	for key, actualVal := range actual {
		if _, exists := expected[key]; !exists {
			diffs[key] = models.TagDiff{
				Expected: "",
				Actual:   actualVal,
				Status:   "added",
			}
		}
	}

	return diffs
}

// deepEqual performs a deep comparison of two values.
func deepEqual(a, b any) bool {
	// Normalize numeric types for comparison (JSON unmarshals numbers as float64)
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return aStr == bStr
}
