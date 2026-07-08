package backend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// FetchStateFile retrieves a Terraform state file from a source.
// Supports:
//   - Local files: "path/to/terraform.tfstate"
//   - S3: "s3://bucket-name/path/to/terraform.tfstate"
//
// Returns the local file path (downloading to a temp location if remote).
func FetchStateFile(ctx context.Context, source string) (string, error) {
	if strings.HasPrefix(source, "s3://") {
		return fetchFromS3(ctx, source)
	}

	// Local file — verify it exists
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return "", fmt.Errorf("state file not found: %s", source)
	}

	return source, nil
}

// fetchFromS3 downloads a state file from S3 to a local temp file.
// Format: s3://bucket-name/path/to/key
func fetchFromS3(ctx context.Context, s3URI string) (string, error) {
	// Parse s3://bucket/key
	path := strings.TrimPrefix(s3URI, "s3://")
	slashIdx := strings.Index(path, "/")
	if slashIdx < 0 {
		return "", fmt.Errorf("invalid S3 URI %q — expected s3://bucket/key", s3URI)
	}

	bucket := path[:slashIdx]
	key := path[slashIdx+1:]

	if bucket == "" || key == "" {
		return "", fmt.Errorf("invalid S3 URI %q — bucket and key must not be empty", s3URI)
	}

	fmt.Printf("☁️  Fetching state from S3: %s/%s\n", bucket, key)

	// Load AWS config from default credential chain
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w\n\nMake sure AWS credentials are configured:\n  - Set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY env vars, or\n  - Configure ~/.aws/credentials, or\n  - Use IAM role if running on EC2/ECS", err)
	}

	client := s3.NewFromConfig(cfg)

	// Download the object
	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return "", fmt.Errorf("failed to download s3://%s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	// Save to temp directory
	tmpDir := filepath.Join(os.TempDir(), "drift-detector")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Use a safe filename from the key
	safeFilename := strings.ReplaceAll(key, "/", "_")
	if !strings.HasSuffix(safeFilename, ".tfstate") {
		safeFilename += ".tfstate"
	}
	localPath := filepath.Join(tmpDir, safeFilename)

	outFile, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	bytes, err := io.Copy(outFile, result.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write state file: %w", err)
	}

	fmt.Printf("✅ Downloaded %d bytes → %s\n\n", bytes, localPath)

	return localPath, nil
}
