package web

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"terraform-drift-detector/internal/backend"
	"terraform-drift-detector/internal/comparator"
	"terraform-drift-detector/internal/models"
	"terraform-drift-detector/internal/parser"
	"terraform-drift-detector/internal/scheduler"
	"terraform-drift-detector/internal/store"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

var (
	appStore     *store.Store
	appScheduler *scheduler.Scheduler
)

// StartServer launches the Gin web server.
func StartServer(addr string, s *store.Store) error {
	appStore = s
	appScheduler = scheduler.New(s)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// API routes
	api := r.Group("/api")
	{
		api.GET("/reports", listReports)
		api.GET("/reports/:id", getReport)
		api.POST("/scans", triggerScan)
		api.GET("/schedules", listSchedules)
		api.POST("/schedules", createSchedule)
		api.POST("/schedules/:id/toggle", toggleSchedule)
		api.GET("/state-files", listStateFiles)
		api.GET("/providers", listProviders)
	}

	// Serve embedded static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to create static sub-filesystem: %w", err)
	}
	r.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	r.GET("/styles.css", func(c *gin.Context) {
		data, err := fs.ReadFile(staticFS, "styles.css")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "text/css; charset=utf-8", data)
	})
	r.GET("/app.js", func(c *gin.Context) {
		data, err := fs.ReadFile(staticFS, "app.js")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", data)
	})

	// Start scheduler for any enabled schedules
	go func() {
		if err := appScheduler.Start(); err != nil {
			fmt.Printf("Scheduler start info: %v\n", err)
		}
	}()

	return r.Run(addr)
}

// listReports returns all saved drift reports.
func listReports(c *gin.Context) {
	reports, err := appStore.ListReports()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reports)
}

// getReport returns a specific drift report by ID.
func getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := appStore.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// triggerScan runs a drift scan on demand.
func triggerScan(c *gin.Context) {
	var req struct {
		StateFile  string `json:"state_file" binding:"required"`
		Provider   string `json:"provider" binding:"required"`
		AWSProfile string `json:"aws_profile"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch state file (supports local files and s3:// URIs)
	localPath, err := backend.FetchStateFile(context.Background(), req.StateFile, req.AWSProfile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to fetch state file: %v", err)})
		return
	}

	// Parse state file
	resources, err := parser.ParseStateFile(localPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse state file: %v", err)})
		return
	}

	// Run comparison
	report, err := comparator.Compare(context.Background(), resources, req.Provider, req.StateFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Scan failed: %v", err)})
		return
	}

	// Save report
	if err := appStore.SaveReport(report); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save report: %v", err)})
		return
	}

	c.JSON(http.StatusOK, report)
}

// listSchedules returns all configured schedules.
func listSchedules(c *gin.Context) {
	schedules, err := appStore.LoadSchedules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, schedules)
}

// createSchedule adds or updates a schedule.
func createSchedule(c *gin.Context) {
	var config models.ScheduleConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if config.ID == "" {
		config.ID = fmt.Sprintf("web-%d", time.Now().Unix())
	}

	// Validate interval
	if _, err := time.ParseDuration(config.Interval); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid interval: %v", err)})
		return
	}

	if err := appScheduler.AddSchedule(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// toggleSchedule enables or disables a schedule.
func toggleSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := appScheduler.ToggleSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "toggled"})
}

// listStateFiles scans testdata/ and current directory for .tfstate files.
func listStateFiles(c *gin.Context) {
	var files []string

	// Search in testdata/
	searchDirs := []string{"testdata", "."}

	for _, dir := range searchDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		entries, err := os.ReadDir(absDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tfstate") {
				files = append(files, filepath.Join(dir, entry.Name()))
			}
		}
	}

	c.JSON(http.StatusOK, files)
}

// listProviders returns available provider names.
func listProviders(c *gin.Context) {
	// Import provider package to ensure all providers are registered
	providers := []string{"mock", "aws"}
	c.JSON(http.StatusOK, providers)
}
