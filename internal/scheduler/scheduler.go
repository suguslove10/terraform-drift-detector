package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"terraform-drift-detector/internal/backend"
	"terraform-drift-detector/internal/comparator"
	"terraform-drift-detector/internal/models"
	"terraform-drift-detector/internal/parser"
	"terraform-drift-detector/internal/store"
)

// Scheduler manages background periodic drift scans.
type Scheduler struct {
	store     *store.Store
	mu        sync.Mutex
	stopChans map[string]chan struct{}
	running   bool
}

// New creates a new Scheduler.
func New(s *store.Store) *Scheduler {
	return &Scheduler{
		store:     s,
		stopChans: make(map[string]chan struct{}),
	}
}

// Start begins executing all enabled schedules.
func (sc *Scheduler) Start() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.running {
		return fmt.Errorf("scheduler is already running")
	}

	schedules, err := sc.store.LoadSchedules()
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	for _, s := range schedules {
		if s.Enabled {
			sc.startSchedule(s)
		}
	}

	sc.running = true
	log.Println("[Scheduler] Started")
	return nil
}

// Stop halts all running schedules.
func (sc *Scheduler) Stop() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for id, ch := range sc.stopChans {
		close(ch)
		delete(sc.stopChans, id)
	}
	sc.running = false
	log.Println("[Scheduler] Stopped")
}

// AddSchedule adds and starts a new schedule.
func (sc *Scheduler) AddSchedule(config models.ScheduleConfig) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Stop existing schedule with same ID if running
	if ch, exists := sc.stopChans[config.ID]; exists {
		close(ch)
		delete(sc.stopChans, config.ID)
	}

	// Save to store
	schedules, _ := sc.store.LoadSchedules()
	found := false
	for i, s := range schedules {
		if s.ID == config.ID {
			schedules[i] = config
			found = true
			break
		}
	}
	if !found {
		schedules = append(schedules, config)
	}
	if err := sc.store.SaveSchedules(schedules); err != nil {
		return err
	}

	if config.Enabled {
		sc.startSchedule(config)
	}

	return nil
}

// ToggleSchedule enables or disables a schedule.
func (sc *Scheduler) ToggleSchedule(id string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	schedules, err := sc.store.LoadSchedules()
	if err != nil {
		return err
	}

	for i, s := range schedules {
		if s.ID == id {
			schedules[i].Enabled = !schedules[i].Enabled

			if schedules[i].Enabled {
				sc.startSchedule(schedules[i])
			} else if ch, exists := sc.stopChans[id]; exists {
				close(ch)
				delete(sc.stopChans, id)
			}

			return sc.store.SaveSchedules(schedules)
		}
	}

	return fmt.Errorf("schedule %q not found", id)
}

// startSchedule launches a goroutine for a single schedule.
func (sc *Scheduler) startSchedule(config models.ScheduleConfig) {
	duration, err := time.ParseDuration(config.Interval)
	if err != nil {
		log.Printf("[Scheduler] Invalid interval %q for schedule %s: %v", config.Interval, config.ID, err)
		return
	}

	stopCh := make(chan struct{})
	sc.stopChans[config.ID] = stopCh

	go func(cfg models.ScheduleConfig) {
		log.Printf("[Scheduler] Schedule %s started (interval: %s, state: %s, provider: %s)",
			cfg.ID, cfg.Interval, cfg.StateFile, cfg.Provider)

		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		// Run immediately on start
		sc.runScan(cfg)

		for {
			select {
			case <-ticker.C:
				sc.runScan(cfg)
			case <-stopCh:
				log.Printf("[Scheduler] Schedule %s stopped", cfg.ID)
				return
			}
		}
	}(config)
}

// runScan executes a drift scan for the given schedule config.
func (sc *Scheduler) runScan(config models.ScheduleConfig) {
	log.Printf("[Scheduler] Running scan for schedule %s (state: %s, provider: %s)",
		config.ID, config.StateFile, config.Provider)

	// Fetch state file (supports local files and s3:// URIs)
	localPath, err := backend.FetchStateFile(context.Background(), config.StateFile, "")
	if err != nil {
		log.Printf("[Scheduler] Failed to fetch state file: %v", err)
		return
	}

	resources, err := parser.ParseStateFile(localPath)
	if err != nil {
		log.Printf("[Scheduler] Failed to parse state file: %v", err)
		return
	}

	report, err := comparator.Compare(context.Background(), resources, config.Provider, config.StateFile)
	if err != nil {
		log.Printf("[Scheduler] Failed to run comparison: %v", err)
		return
	}

	if err := sc.store.SaveReport(report); err != nil {
		log.Printf("[Scheduler] Failed to save report: %v", err)
		return
	}

	log.Printf("[Scheduler] Scan complete: %d total, %d in-sync, %d drifted, %d deleted",
		report.TotalResources, report.InSyncCount, report.DriftedCount, report.DeletedCount)

	// Update last run time
	sc.mu.Lock()
	defer sc.mu.Unlock()
	schedules, _ := sc.store.LoadSchedules()
	for i, s := range schedules {
		if s.ID == config.ID {
			schedules[i].LastRun = time.Now().Format(time.RFC3339)
			sc.store.SaveSchedules(schedules)
			break
		}
	}
}
