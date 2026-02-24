package tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFISModuleFullDeployment provisions one target resource per supported service
// (S3, Kinesis, DynamoDB, Lambda, Network) using internal modules, invokes the
// FIS_Module with 5 experiment templates, and validates all outputs.
//
// Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9
func TestFISModuleFullDeployment(t *testing.T) {
	t.Parallel()

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "./fixtures",
	})

	// Req 10.9: Cleanup after test execution
	defer terraform.Destroy(t, terraformOptions)
	terraform.InitAndApply(t, terraformOptions)

	// -----------------------------------------------------------------------
	// Req 10.7: Validate experiment_role_arn references FISExperimentRole
	// -----------------------------------------------------------------------
	experimentRoleArn := terraform.Output(t, terraformOptions, "experiment_role_arn")
	require.NotEmpty(t, experimentRoleArn)
	assert.Contains(t, experimentRoleArn, "FISExperimentRole",
		"experiment_role_arn should reference the FISExperimentRole IAM role")

	// -----------------------------------------------------------------------
	// Req 10.4: Validate S3 bucket name and ARN outputs
	// -----------------------------------------------------------------------
	s3BucketName := terraform.Output(t, terraformOptions, "s3_bucket_name")
	require.NotEmpty(t, s3BucketName)
	assert.True(t, strings.HasPrefix(s3BucketName, "fis-lambda-config-"),
		"S3 bucket name should start with 'fis-lambda-config-'")

	s3BucketArn := terraform.Output(t, terraformOptions, "s3_bucket_arn")
	require.NotEmpty(t, s3BucketArn)
	assert.Contains(t, s3BucketArn, s3BucketName,
		"S3 bucket ARN should contain the bucket name")

	// -----------------------------------------------------------------------
	// Req 10.5: Validate KMS key ID and ARN outputs
	// -----------------------------------------------------------------------
	kmsKeyID := terraform.Output(t, terraformOptions, "kms_key_id")
	require.NotEmpty(t, kmsKeyID, "KMS key ID should be non-empty")

	kmsKeyArn := terraform.Output(t, terraformOptions, "kms_key_arn")
	require.NotEmpty(t, kmsKeyArn, "KMS key ARN should be non-empty")
	assert.Contains(t, kmsKeyArn, "kms",
		"KMS key ARN should contain 'kms'")

	// -----------------------------------------------------------------------
	// Req 10.6: Validate shared CloudWatch log group name and ARN outputs
	// -----------------------------------------------------------------------
	logGroupName := terraform.Output(t, terraformOptions, "log_group_name")
	require.NotEmpty(t, logGroupName)
	assert.Equal(t, "/aws/fis/experiments/test", logGroupName,
		"Log group name should follow /aws/fis/experiments/{environment} pattern")

	logGroupArn := terraform.Output(t, terraformOptions, "log_group_arn")
	require.NotEmpty(t, logGroupArn)
	assert.Contains(t, logGroupArn, logGroupName,
		"Log group ARN should contain the log group name")

	// -----------------------------------------------------------------------
	// Req 10.3, 10.8: Validate experiment templates for each service target
	// -----------------------------------------------------------------------
	experimentTemplates := terraform.OutputMapOfObjects(t, terraformOptions, "experiment_templates")

	expectedTemplateKeys := []string{
		"s3-pause-replication",
		"kinesis-deactivate-stream",
		"dynamodb-pause-replication",
		"lambda-inject-fault",
		"network-disrupt-connectivity",
	}

	// Verify one template per service target
	assert.Equal(t, len(expectedTemplateKeys), len(experimentTemplates),
		"Should have exactly one experiment template per service target")

	for _, key := range expectedTemplateKeys {
		tpl, ok := experimentTemplates[key]
		require.True(t, ok, "Expected template key %q in experiment_templates output", key)

		tplMap, ok := tpl.(map[string]interface{})
		require.True(t, ok, "Template %q should be a map", key)

		// Assert non-empty id
		id, ok := tplMap["id"].(string)
		require.True(t, ok, "Template %q should have a string 'id' field", key)
		assert.NotEmpty(t, id, "Template %q should have non-empty id", key)

		// Assert ARN follows arn:aws:fis:{region}:{account}:experiment-template/{id}
		arn, ok := tplMap["arn"].(string)
		require.True(t, ok, "Template %q should have a string 'arn' field", key)
		assert.NotEmpty(t, arn, "Template %q should have non-empty arn", key)
		assert.Contains(t, arn, "arn:aws:fis:",
			"Template %q ARN should start with arn:aws:fis:", key)
		assert.Contains(t, arn, fmt.Sprintf("experiment-template/%s", id),
			"Template %q ARN should contain experiment-template/{id}", key)

		// Assert name follows fis-{key}-{environment} convention
		name, ok := tplMap["name"].(string)
		require.True(t, ok, "Template %q should have a string 'name' field", key)
		expectedName := fmt.Sprintf("fis-%s-test", key)
		assert.Equal(t, expectedName, name,
			"Template %q name should follow fis-{key}-{environment} convention", key)
	}
}
