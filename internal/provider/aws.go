package provider

import (
	"context"
	"fmt"

	"terraform-drift-detector/internal/models"
)

// AWSProvider fetches actual resource state from AWS using the AWS SDK.
// Currently a stub — requires aws-sdk-go-v2 dependency for full implementation.
type AWSProvider struct{}

func init() {
	Register("aws", &AWSProvider{})
}

func (a *AWSProvider) Name() string {
	return "aws"
}

func (a *AWSProvider) SupportedTypes() []string {
	return []string{
		"aws_instance",
		"aws_s3_bucket",
		"aws_security_group",
		"aws_iam_role",
	}
}

func (a *AWSProvider) FetchActual(ctx context.Context, resourceType, resourceID string, expectedAttrs map[string]any, expectedTags map[string]string) (*models.ResourceState, error) {
	// TODO: Implement actual AWS API calls using aws-sdk-go-v2
	// For now, return an error indicating the provider needs configuration.
	//
	// When implemented, this would:
	//   - aws_instance -> ec2.DescribeInstances
	//   - aws_s3_bucket -> s3.HeadBucket + GetBucketTagging + GetBucketVersioning
	//   - aws_security_group -> ec2.DescribeSecurityGroups
	//   - aws_iam_role -> iam.GetRole

	return nil, fmt.Errorf("AWS provider not yet configured — use 'mock' provider for demos, or implement AWS SDK calls for resource type %q", resourceType)
}
