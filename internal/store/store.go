package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"terraform-drift-detector/internal/models"
)

const (
	dataDir       = ".drift_data"
	reportsDir    = "reports"
	schedulesFile = "schedules.json"
)

// Store handles persistence of drift reports and schedule configs.
type Store struct {
	basePath string
}

// New creates a new Store at the given base path.
func New(basePath string) *Store {
	return &Store{basePath: basePath}
}

// Default creates a Store using the current working directory.
func Default() *Store {
	cwd, _ := os.Getwd()
	return New(filepath.Join(cwd, dataDir))
}

// ensureDir creates the directory if it doesn't exist.
func (s *Store) ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// reportsPath returns the path to the reports directory.
func (s *Store) reportsPath() string {
	return filepath.Join(s.basePath, reportsDir)
}

// SaveReport persists a DriftReport as a JSON file.
func (s *Store) SaveReport(report *models.DriftReport) error {
	dir := s.reportsPath()
	if err := s.ensureDir(dir); err != nil {
		return fmt.Errorf("failed to create reports dir: %w", err)
	}

	filename := fmt.Sprintf("report-%s.json", report.ID)
	filePath := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// ListReports returns all saved drift reports, sorted by timestamp (newest first).
func (s *Store) ListReports() ([]models.DriftReport, error) {
	dir := s.reportsPath()
	if err := s.ensureDir(dir); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read reports dir: %w", err)
	}

	var reports []models.DriftReport
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var report models.DriftReport
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}

		reports = append(reports, report)
	}

	// Sort by timestamp descending
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Timestamp > reports[j].Timestamp
	})

	return reports, nil
}

// GetReport retrieves a specific drift report by ID.
func (s *Store) GetReport(id string) (*models.DriftReport, error) {
	dir := s.reportsPath()
	filename := fmt.Sprintf("report-%s.json", id)
	filePath := filepath.Join(dir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("report %q not found: %w", id, err)
	}

	var report models.DriftReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report: %w", err)
	}

	return &report, nil
}

// SaveSchedules persists the schedule configs.
func (s *Store) SaveSchedules(schedules []models.ScheduleConfig) error {
	if err := s.ensureDir(s.basePath); err != nil {
		return err
	}

	data, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	return os.WriteFile(filepath.Join(s.basePath, schedulesFile), data, 0644)
}

// LoadSchedules loads the schedule configs from disk.
func (s *Store) LoadSchedules() ([]models.ScheduleConfig, error) {
	filePath := filepath.Join(s.basePath, schedulesFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.ScheduleConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read schedules: %w", err)
	}

	var schedules []models.ScheduleConfig
	if err := json.Unmarshal(data, &schedules); err != nil {
		return nil, fmt.Errorf("failed to parse schedules: %w", err)
	}

	return schedules, nil
}
