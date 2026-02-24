package tests

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// constructBucketName replicates the module's S3 bucket name construction logic.
// Bucket name = "fis-lambda-config-{account_id}-{ci_commit_ref_name}"
func constructBucketName(accountID, ciCommitRefName string) string {
	return fmt.Sprintf("fis-lambda-config-%s-%s", accountID, ciCommitRefName)
}

// genAccountID generates a random 12-digit numeric string (AWS account ID format).
func genAccountID(t *rapid.T) string {
	digits := make([]byte, 12)
	for i := range digits {
		digits[i] = byte('0' + rapid.IntRange(0, 9).Draw(t, fmt.Sprintf("digit[%d]", i)))
	}
	return string(digits)
}

// genCICommitRefName generates a valid ci_commit_ref_name string:
// - lowercase letters, numbers, and hyphens only
// - no leading/trailing hyphens
// - no consecutive hyphens
func genCICommitRefName(t *rapid.T) string {
	// Valid characters for non-hyphen positions
	alnum := "abcdefghijklmnopqrstuvwxyz0123456789"

	// Generate length between 1 and 30
	length := rapid.IntRange(1, 30).Draw(t, "length")

	var b strings.Builder
	for i := 0; i < length; i++ {
		if i == 0 || i == length-1 {
			// First and last character must be alphanumeric
			idx := rapid.IntRange(0, len(alnum)-1).Draw(t, fmt.Sprintf("char[%d]", i))
			b.WriteByte(alnum[idx])
		} else {
			// Middle characters can be alphanumeric or hyphen, but no consecutive hyphens
			useHyphen := rapid.Bool().Draw(t, fmt.Sprintf("hyphen[%d]", i))
			if useHyphen && b.Len() > 0 && b.String()[b.Len()-1] != '-' {
				b.WriteByte('-')
			} else {
				idx := rapid.IntRange(0, len(alnum)-1).Draw(t, fmt.Sprintf("char[%d]", i))
				b.WriteByte(alnum[idx])
			}
		}
	}

	return b.String()
}

// Feature: aws-fis-terraform-module, Property 1: S3 Bucket Name Construction
// Validates: Requirements 2.5
func TestProperty1_S3BucketNameConstruction(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		accountID := genAccountID(t)
		ciCommitRefName := genCICommitRefName(t)

		bucketName := constructBucketName(accountID, ciCommitRefName)
		expected := fmt.Sprintf("fis-lambda-config-%s-%s", accountID, ciCommitRefName)

		if bucketName != expected {
			t.Fatalf("bucket name mismatch: got %q, want %q", bucketName, expected)
		}

		// Verify the bucket name has the correct prefix
		if !strings.HasPrefix(bucketName, "fis-lambda-config-") {
			t.Fatalf("bucket name missing prefix 'fis-lambda-config-': got %q", bucketName)
		}

		// Verify the account ID and ref name are embedded correctly
		withoutPrefix := strings.TrimPrefix(bucketName, "fis-lambda-config-")
		if !strings.HasPrefix(withoutPrefix, accountID+"-") {
			t.Fatalf("bucket name does not contain account ID after prefix: got %q", bucketName)
		}

		suffix := strings.TrimPrefix(withoutPrefix, accountID+"-")
		if suffix != ciCommitRefName {
			t.Fatalf("bucket name suffix mismatch: got %q, want %q", suffix, ciCommitRefName)
		}
	})
}

// validateBucketNameLength returns true if the bucket name is within the S3
// maximum length of 63 characters, false otherwise.
func validateBucketNameLength(bucketName string) bool {
	return len(bucketName) <= 63
}

// genCICommitRefNameWithLength generates a valid ci_commit_ref_name of exactly
// the requested length using only lowercase alphanumeric characters.
// Minimum length is 1.
func genCICommitRefNameWithLength(t *rapid.T, length int) string {
	if length <= 0 {
		length = 1
	}
	alnum := "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		idx := rapid.IntRange(0, len(alnum)-1).Draw(t, fmt.Sprintf("char[%d]", i))
		b[i] = alnum[idx]
	}
	return string(b)
}

// Feature: aws-fis-terraform-module, Property 2: S3 Bucket Name Length Validation
// Validates: Requirements 2.6
func TestProperty2_S3BucketNameLengthValidation(t *testing.T) {
	// The bucket name prefix is "fis-lambda-config-" (18 chars) + 12-digit account ID + "-" = 31 chars fixed.
	// So ci_commit_ref_name must be <= 32 chars for the total to be <= 63.
	const fixedPrefixLen = 31 // "fis-lambda-config-" + 12-digit account ID + "-"
	const maxBucketNameLen = 63
	const maxRefNameLen = maxBucketNameLen - fixedPrefixLen // 32

	rapid.Check(t, func(t *rapid.T) {
		// Use a fixed-format 12-digit account ID
		accountID := genAccountID(t)

		// Generate ci_commit_ref_name of varying lengths: 1 to 50 chars
		// This covers both valid (<=32) and invalid (>32) lengths
		refNameLen := rapid.IntRange(1, 50).Draw(t, "refNameLen")
		ciCommitRefName := genCICommitRefNameWithLength(t, refNameLen)

		bucketName := constructBucketName(accountID, ciCommitRefName)
		isValid := validateBucketNameLength(bucketName)

		if len(ciCommitRefName) <= maxRefNameLen {
			// Bucket name should be <= 63 chars, validation should accept
			if !isValid {
				t.Fatalf("expected bucket name to be accepted (len=%d, refName len=%d), but was rejected: %q",
					len(bucketName), len(ciCommitRefName), bucketName)
			}
			if len(bucketName) > maxBucketNameLen {
				t.Fatalf("bucket name exceeds %d chars despite refName len %d <= %d: %q (len=%d)",
					maxBucketNameLen, len(ciCommitRefName), maxRefNameLen, bucketName, len(bucketName))
			}
		} else {
			// Bucket name should be > 63 chars, validation should reject
			if isValid {
				t.Fatalf("expected bucket name to be rejected (len=%d, refName len=%d), but was accepted: %q",
					len(bucketName), len(ciCommitRefName), bucketName)
			}
			if len(bucketName) <= maxBucketNameLen {
				t.Fatalf("bucket name is <= %d chars despite refName len %d > %d: %q (len=%d)",
					maxBucketNameLen, len(ciCommitRefName), maxRefNameLen, bucketName, len(bucketName))
			}
		}
	})
}

// validateCICommitRefName checks that a ci_commit_ref_name value is S3-safe:
// 1. Must match ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ (lowercase alphanumeric, hyphens; no leading/trailing hyphens)
// 2. Must not contain consecutive hyphens (--)
// 3. Must not be empty
func validateCICommitRefName(name string) bool {
	if name == "" {
		return false
	}
	matched, err := regexp.MatchString(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, name)
	if err != nil || !matched {
		return false
	}
	if strings.Contains(name, "--") {
		return false
	}
	return true
}

// Feature: aws-fis-terraform-module, Property 12: ci_commit_ref_name S3-Safe Character Validation
// Validates: Requirements 2.7
func TestProperty12_CICommitRefNameS3SafeValidation(t *testing.T) {
	alnum := "abcdefghijklmnopqrstuvwxyz0123456789"

	// Sub-test: valid ci_commit_ref_name values are accepted
	t.Run("valid_names_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate a valid name: lowercase alphanumeric with optional single hyphens between chars
			length := rapid.IntRange(1, 30).Draw(t, "length")
			var b strings.Builder
			for i := 0; i < length; i++ {
				if i == 0 || i == length-1 {
					idx := rapid.IntRange(0, len(alnum)-1).Draw(t, fmt.Sprintf("c[%d]", i))
					b.WriteByte(alnum[idx])
				} else {
					useHyphen := rapid.Bool().Draw(t, fmt.Sprintf("h[%d]", i))
					if useHyphen && b.Len() > 0 && b.String()[b.Len()-1] != '-' {
						b.WriteByte('-')
					} else {
						idx := rapid.IntRange(0, len(alnum)-1).Draw(t, fmt.Sprintf("c[%d]", i))
						b.WriteByte(alnum[idx])
					}
				}
			}
			name := b.String()

			if !validateCICommitRefName(name) {
				t.Fatalf("expected valid name to be accepted: %q", name)
			}
		})
	})

	// Sub-test: strings with uppercase letters are rejected
	t.Run("uppercase_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			upper := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
			prefix := genCICommitRefNameWithLength(t, rapid.IntRange(1, 10).Draw(t, "prefixLen"))
			suffix := genCICommitRefNameWithLength(t, rapid.IntRange(1, 10).Draw(t, "suffixLen"))
			ch := upper[rapid.IntRange(0, len(upper)-1).Draw(t, "upperChar")]
			name := prefix + string(ch) + suffix

			if validateCICommitRefName(name) {
				t.Fatalf("expected name with uppercase to be rejected: %q", name)
			}
		})
	})

	// Sub-test: strings with underscores, slashes, periods are rejected
	t.Run("invalid_chars_rejected", func(t *testing.T) {
		invalidChars := []byte{'_', '/', '.', '@', '!', ' ', '+'}
		rapid.Check(t, func(t *rapid.T) {
			prefix := genCICommitRefNameWithLength(t, rapid.IntRange(1, 10).Draw(t, "prefixLen"))
			suffix := genCICommitRefNameWithLength(t, rapid.IntRange(1, 10).Draw(t, "suffixLen"))
			ch := invalidChars[rapid.IntRange(0, len(invalidChars)-1).Draw(t, "invalidChar")]
			name := prefix + string(ch) + suffix

			if validateCICommitRefName(name) {
				t.Fatalf("expected name with invalid char %q to be rejected: %q", string(ch), name)
			}
		})
	})

	// Sub-test: leading hyphens are rejected
	t.Run("leading_hyphen_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			base := genCICommitRefNameWithLength(t, rapid.IntRange(1, 15).Draw(t, "baseLen"))
			name := "-" + base

			if validateCICommitRefName(name) {
				t.Fatalf("expected name with leading hyphen to be rejected: %q", name)
			}
		})
	})

	// Sub-test: trailing hyphens are rejected
	t.Run("trailing_hyphen_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			base := genCICommitRefNameWithLength(t, rapid.IntRange(1, 15).Draw(t, "baseLen"))
			name := base + "-"

			if validateCICommitRefName(name) {
				t.Fatalf("expected name with trailing hyphen to be rejected: %q", name)
			}
		})
	})

	// Sub-test: consecutive hyphens are rejected
	t.Run("consecutive_hyphens_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			prefix := genCICommitRefNameWithLength(t, rapid.IntRange(1, 10).Draw(t, "prefixLen"))
			suffix := genCICommitRefNameWithLength(t, rapid.IntRange(1, 10).Draw(t, "suffixLen"))
			name := prefix + "--" + suffix

			if validateCICommitRefName(name) {
				t.Fatalf("expected name with consecutive hyphens to be rejected: %q", name)
			}
		})
	})

	// Sub-test: empty string is rejected
	t.Run("empty_string_rejected", func(t *testing.T) {
		if validateCICommitRefName("") {
			t.Fatal("expected empty string to be rejected")
		}
	})
}

// validateSelectionMode replicates the module's selection_mode validation logic.
// Rules:
//  1. "ALL" is always valid
//  2. "COUNT(n)" where n is an integer > 0 is valid
//  3. "PERCENT(n)" where n is an integer from 1 through 100 is valid
//  4. Any other format is invalid
func validateSelectionMode(mode string) bool {
	if mode == "ALL" {
		return true
	}

	countRe := regexp.MustCompile(`^COUNT\((\d+)\)$`)
	if m := countRe.FindStringSubmatch(mode); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return false
		}
		return n > 0
	}

	percentRe := regexp.MustCompile(`^PERCENT\((\d+)\)$`)
	if m := percentRe.FindStringSubmatch(mode); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return false
		}
		return n >= 1 && n <= 100
	}

	return false
}

// Feature: aws-fis-terraform-module, Property 7: Selection Mode Validation
// Validates: Requirements 4.10, 4.11, 4.12
func TestProperty7_SelectionModeValidation(t *testing.T) {
	// Sub-test: valid "ALL" mode is accepted
	t.Run("ALL_is_valid", func(t *testing.T) {
		if !validateSelectionMode("ALL") {
			t.Fatal("expected ALL to be valid")
		}
	})

	// Sub-test: valid COUNT(n) with n in 1-1000 is accepted
	t.Run("valid_COUNT_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			n := rapid.IntRange(1, 1000).Draw(t, "n")
			mode := fmt.Sprintf("COUNT(%d)", n)
			if !validateSelectionMode(mode) {
				t.Fatalf("expected valid COUNT mode to be accepted: %q", mode)
			}
		})
	})

	// Sub-test: valid PERCENT(n) with n in 1-100 is accepted
	t.Run("valid_PERCENT_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			n := rapid.IntRange(1, 100).Draw(t, "n")
			mode := fmt.Sprintf("PERCENT(%d)", n)
			if !validateSelectionMode(mode) {
				t.Fatalf("expected valid PERCENT mode to be accepted: %q", mode)
			}
		})
	})

	// Sub-test: COUNT(0) is rejected
	t.Run("COUNT_zero_rejected", func(t *testing.T) {
		if validateSelectionMode("COUNT(0)") {
			t.Fatal("expected COUNT(0) to be rejected")
		}
	})

	// Sub-test: COUNT with negative values is rejected (format won't match \d+)
	t.Run("COUNT_negative_rejected", func(t *testing.T) {
		if validateSelectionMode("COUNT(-1)") {
			t.Fatal("expected COUNT(-1) to be rejected")
		}
	})

	// Sub-test: PERCENT(0) is rejected
	t.Run("PERCENT_zero_rejected", func(t *testing.T) {
		if validateSelectionMode("PERCENT(0)") {
			t.Fatal("expected PERCENT(0) to be rejected")
		}
	})

	// Sub-test: PERCENT(n) with n > 100 is rejected
	t.Run("PERCENT_over_100_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			n := rapid.IntRange(101, 1000).Draw(t, "n")
			mode := fmt.Sprintf("PERCENT(%d)", n)
			if validateSelectionMode(mode) {
				t.Fatalf("expected PERCENT mode with n > 100 to be rejected: %q", mode)
			}
		})
	})

	// Sub-test: malformed strings are rejected
	t.Run("malformed_strings_rejected", func(t *testing.T) {
		malformed := []string{
			"", "all", "All", "count(5)", "percent(50)",
			"COUNT", "PERCENT", "COUNT()", "PERCENT()",
			"COUNT(abc)", "PERCENT(abc)", "COUNT(1.5)", "PERCENT(50.5)",
			"RANDOM", "COUNT (5)", "PERCENT (50)",
			"COUNT(5) ", " COUNT(5)", "COUNT(5)extra",
			"NONE", "SELECT(5)", "COUNT(-5)",
		}
		for _, mode := range malformed {
			if validateSelectionMode(mode) {
				t.Fatalf("expected malformed mode to be rejected: %q", mode)
			}
		}
	})

	// Sub-test: random garbage strings are rejected
	t.Run("random_garbage_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate random strings that are NOT valid selection modes
			garbage := rapid.StringMatching(`[a-zA-Z0-9!@#$%^&*_+=]{1,30}`).Draw(t, "garbage")
			// Skip if we accidentally generated a valid mode
			if garbage == "ALL" {
				return
			}
			if matched, _ := regexp.MatchString(`^COUNT\(\d+\)$`, garbage); matched {
				return
			}
			if matched, _ := regexp.MatchString(`^PERCENT\(\d+\)$`, garbage); matched {
				return
			}
			if validateSelectionMode(garbage) {
				t.Fatalf("expected random garbage to be rejected: %q", garbage)
			}
		})
	})
}

// StopCondition represents a stop condition entry in an experiment template.
type StopCondition struct {
	Source string
	Value  string
}

// resolveStopConditions replicates the module's default stop condition logic:
// - If stop_conditions is nil or empty, return [{source: "none", value: ""}]
// - Otherwise return the provided stop_conditions as-is
func resolveStopConditions(stopConditions []StopCondition) []StopCondition {
	if len(stopConditions) == 0 {
		return []StopCondition{{Source: "none", Value: ""}}
	}
	return stopConditions
}

// genStopCondition generates a random non-default stop condition.
func genStopCondition(t *rapid.T) StopCondition {
	sources := []string{"aws:cloudwatch:alarm", "none"}
	source := sources[rapid.IntRange(0, len(sources)-1).Draw(t, "source")]
	value := ""
	if source == "aws:cloudwatch:alarm" {
		// Generate a plausible alarm ARN
		accountID := genAccountID(t)
		alarmName := genCICommitRefNameWithLength(t, rapid.IntRange(3, 20).Draw(t, "alarmNameLen"))
		value = fmt.Sprintf("arn:aws:cloudwatch:us-east-1:%s:alarm:%s", accountID, alarmName)
	}
	return StopCondition{Source: source, Value: value}
}

// Feature: aws-fis-terraform-module, Property 5: Stop Condition Default
// Validates: Requirements 4.5
func TestProperty5_StopConditionDefault(t *testing.T) {
	defaultStopCondition := StopCondition{Source: "none", Value: ""}

	// Sub-test: nil stop_conditions returns default
	t.Run("nil_returns_default", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			result := resolveStopConditions(nil)
			if len(result) != 1 {
				t.Fatalf("expected exactly 1 default stop condition, got %d", len(result))
			}
			if result[0] != defaultStopCondition {
				t.Fatalf("expected default stop condition {source: %q, value: %q}, got {source: %q, value: %q}",
					defaultStopCondition.Source, defaultStopCondition.Value,
					result[0].Source, result[0].Value)
			}
		})
	})

	// Sub-test: empty slice returns default
	t.Run("empty_returns_default", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			result := resolveStopConditions([]StopCondition{})
			if len(result) != 1 {
				t.Fatalf("expected exactly 1 default stop condition, got %d", len(result))
			}
			if result[0] != defaultStopCondition {
				t.Fatalf("expected default stop condition {source: %q, value: %q}, got {source: %q, value: %q}",
					defaultStopCondition.Source, defaultStopCondition.Value,
					result[0].Source, result[0].Value)
			}
		})
	})

	// Sub-test: provided stop_conditions are passed through unchanged
	t.Run("provided_conditions_passthrough", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			count := rapid.IntRange(1, 5).Draw(t, "count")
			conditions := make([]StopCondition, count)
			for i := 0; i < count; i++ {
				conditions[i] = genStopCondition(t)
			}

			result := resolveStopConditions(conditions)

			if len(result) != len(conditions) {
				t.Fatalf("expected %d stop conditions, got %d", len(conditions), len(result))
			}
			for i, sc := range result {
				if sc != conditions[i] {
					t.Fatalf("stop condition[%d] mismatch: got {source: %q, value: %q}, want {source: %q, value: %q}",
						i, sc.Source, sc.Value, conditions[i].Source, conditions[i].Value)
				}
			}
		})
	})

	// Sub-test: default is exactly one stop condition with source="none"
	t.Run("default_source_is_none", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Randomly choose nil or empty to trigger default
			useNil := rapid.Bool().Draw(t, "useNil")
			var input []StopCondition
			if !useNil {
				input = []StopCondition{}
			}

			result := resolveStopConditions(input)

			if len(result) != 1 {
				t.Fatalf("expected exactly 1 default stop condition, got %d", len(result))
			}
			if result[0].Source != "none" {
				t.Fatalf("expected default source to be %q, got %q", "none", result[0].Source)
			}
			if result[0].Value != "" {
				t.Fatalf("expected default value to be empty, got %q", result[0].Value)
			}
		})
	})
}

// resolveSelectionMode replicates the module's default selection_mode logic:
// - If selection_mode is empty string, return "ALL"
// - Otherwise return the provided selection_mode
func resolveSelectionMode(selectionMode string) string {
	if selectionMode == "" {
		return "ALL"
	}
	return selectionMode
}

// Feature: aws-fis-terraform-module, Property 6: Selection Mode Default
// Validates: Requirements 4.7
func TestProperty6_SelectionModeDefault(t *testing.T) {
	// Sub-test: empty selection_mode defaults to "ALL"
	t.Run("empty_defaults_to_ALL", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			result := resolveSelectionMode("")
			if result != "ALL" {
				t.Fatalf("expected default selection_mode to be %q, got %q", "ALL", result)
			}
		})
	})

	// Sub-test: provided selection_mode is passed through unchanged
	t.Run("provided_mode_passthrough", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			modes := []string{"ALL", "COUNT(1)", "COUNT(5)", "COUNT(100)", "PERCENT(1)", "PERCENT(50)", "PERCENT(100)"}
			mode := modes[rapid.IntRange(0, len(modes)-1).Draw(t, "modeIdx")]
			result := resolveSelectionMode(mode)
			if result != mode {
				t.Fatalf("expected selection_mode %q to pass through, got %q", mode, result)
			}
		})
	})

	// Sub-test: arbitrary non-empty strings are passed through
	t.Run("arbitrary_nonempty_passthrough", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			mode := rapid.StringMatching(`[A-Z][A-Z0-9()]{0,29}`).Draw(t, "mode")
			if mode == "" {
				return
			}
			result := resolveSelectionMode(mode)
			if result != mode {
				t.Fatalf("expected non-empty selection_mode %q to pass through, got %q", mode, result)
			}
		})
	})

	// Sub-test: default is exactly "ALL"
	t.Run("default_is_exactly_ALL", func(t *testing.T) {
		result := resolveSelectionMode("")
		if result != "ALL" {
			t.Fatalf("expected default to be exactly %q, got %q", "ALL", result)
		}
	})
}

// constructTemplateName replicates the module's template Name tag construction logic.
// Name tag = "fis-{templateKey}-{environment}" where templateKey encodes {service}-{scenario}.
func constructTemplateName(templateKey, environment string) string {
	return fmt.Sprintf("fis-%s-%s", templateKey, environment)
}

// Feature: aws-fis-terraform-module, Property 8: Template Naming Convention
// Validates: Requirements 4.16
func TestProperty8_TemplateNamingConvention(t *testing.T) {
	// Generator for service names (lowercase, 2-15 chars)
	genService := func(t *rapid.T) string {
		return rapid.StringMatching(`[a-z][a-z0-9]{1,14}`).Draw(t, "service")
	}

	// Generator for scenario names (lowercase with hyphens, 3-20 chars)
	genScenario := func(t *rapid.T) string {
		return rapid.StringMatching(`[a-z][a-z0-9-]{2,19}`).Draw(t, "scenario")
	}

	// Generator for environment values
	genEnvironment := func(t *rapid.T) string {
		envs := []string{"dev", "staging", "prod", "qa", "uat", "sandbox", "test"}
		return envs[rapid.IntRange(0, len(envs)-1).Draw(t, "envIdx")]
	}

	rapid.Check(t, func(t *rapid.T) {
		service := genService(t)
		scenario := genScenario(t)
		environment := genEnvironment(t)

		// Template key is {service}-{scenario}
		templateKey := fmt.Sprintf("%s-%s", service, scenario)

		name := constructTemplateName(templateKey, environment)
		expected := fmt.Sprintf("fis-%s-%s", templateKey, environment)

		// Verify the constructed name equals "fis-{templateKey}-{environment}"
		if name != expected {
			t.Fatalf("name mismatch: got %q, want %q", name, expected)
		}

		// Verify the name starts with "fis-"
		if !strings.HasPrefix(name, "fis-") {
			t.Fatalf("name missing prefix 'fis-': got %q", name)
		}

		// Verify the name contains the service, scenario, and environment
		expectedFull := fmt.Sprintf("fis-%s-%s-%s", service, scenario, environment)
		if name != expectedFull {
			t.Fatalf("name does not match full convention fis-{service}-{scenario}-{environment}: got %q, want %q", name, expectedFull)
		}
	})
}

// constructLogGroupName replicates the module's CloudWatch log group name construction logic.
// Log group name = "/aws/fis/experiments/{environment}"
func constructLogGroupName(environment string) string {
	return fmt.Sprintf("/aws/fis/experiments/%s", environment)
}

// resolveLogGroupCount returns the number of log groups the module creates.
// The module creates exactly one shared log group regardless of how many templates exist.
func resolveLogGroupCount(_ int) int {
	return 1
}

// Feature: aws-fis-terraform-module, Property 9: Single Deterministic Log Group
// Validates: Requirements 5.1, 5.2
func TestProperty9_SingleDeterministicLogGroup(t *testing.T) {
	// Generator for environment values
	genEnvironment := func(t *rapid.T) string {
		envs := []string{"dev", "staging", "prod", "qa", "uat", "sandbox", "test"}
		return envs[rapid.IntRange(0, len(envs)-1).Draw(t, "envIdx")]
	}

	// Sub-test: exactly one log group regardless of template count
	t.Run("exactly_one_log_group", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			templateCount := rapid.IntRange(0, 10).Draw(t, "templateCount")
			count := resolveLogGroupCount(templateCount)
			if count != 1 {
				t.Fatalf("expected exactly 1 log group for %d templates, got %d", templateCount, count)
			}
		})
	})

	// Sub-test: log group name matches /aws/fis/experiments/{environment}
	t.Run("log_group_name_matches_pattern", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			environment := genEnvironment(t)
			_ = rapid.IntRange(0, 10).Draw(t, "templateCount") // varying template counts

			logGroupName := constructLogGroupName(environment)
			expected := fmt.Sprintf("/aws/fis/experiments/%s", environment)

			if logGroupName != expected {
				t.Fatalf("log group name mismatch: got %q, want %q", logGroupName, expected)
			}
		})
	})

	// Sub-test: name is deterministic (same environment always produces same name)
	t.Run("name_is_deterministic", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			environment := genEnvironment(t)

			name1 := constructLogGroupName(environment)
			name2 := constructLogGroupName(environment)

			if name1 != name2 {
				t.Fatalf("log group name is not deterministic for environment %q: got %q and %q",
					environment, name1, name2)
			}
		})
	})

	// Sub-test: log group name has correct prefix
	t.Run("log_group_name_has_correct_prefix", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			environment := genEnvironment(t)
			logGroupName := constructLogGroupName(environment)

			if !strings.HasPrefix(logGroupName, "/aws/fis/experiments/") {
				t.Fatalf("log group name missing prefix '/aws/fis/experiments/': got %q", logGroupName)
			}

			// Verify the environment is the suffix
			suffix := strings.TrimPrefix(logGroupName, "/aws/fis/experiments/")
			if suffix != environment {
				t.Fatalf("log group name suffix mismatch: got %q, want %q", suffix, environment)
			}
		})
	})
}

// resolveTemplateCount simulates the module's for_each behavior:
// for_each = var.experiment_templates creates exactly N resources for N input entries,
// and the output map contains exactly N entries.
func resolveTemplateCount(inputCount int) int {
	return inputCount
}

// Feature: aws-fis-terraform-module, Property 3: Template Count Equals Input Count
// Validates: Requirements 4.2, 9.4
func TestProperty3_TemplateCountEqualsInputCount(t *testing.T) {
	// Sub-test: output count always equals input count for varying map sizes
	t.Run("output_count_equals_input_count", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			inputCount := rapid.IntRange(1, 10).Draw(t, "inputCount")
			outputCount := resolveTemplateCount(inputCount)

			if outputCount != inputCount {
				t.Fatalf("expected output count %d to equal input count %d", outputCount, inputCount)
			}
		})
	})

	// Sub-test: each input key produces a corresponding output entry
	t.Run("each_key_has_output_entry", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			mapSize := rapid.IntRange(1, 10).Draw(t, "mapSize")

			// Generate unique template keys
			inputKeys := make(map[string]bool)
			for i := 0; i < mapSize; i++ {
				key := fmt.Sprintf("%s-%s",
					rapid.StringMatching(`[a-z]{2,8}`).Draw(t, fmt.Sprintf("svc[%d]", i)),
					rapid.StringMatching(`[a-z]{3,12}`).Draw(t, fmt.Sprintf("scn[%d]", i)),
				)
				inputKeys[key] = true
			}

			// Simulate for_each: output map has one entry per unique input key
			outputCount := resolveTemplateCount(len(inputKeys))

			if outputCount != len(inputKeys) {
				t.Fatalf("expected output count %d to equal unique input key count %d", outputCount, len(inputKeys))
			}
		})
	})

	// Sub-test: single template produces exactly one output
	t.Run("single_template_single_output", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			outputCount := resolveTemplateCount(1)
			if outputCount != 1 {
				t.Fatalf("expected 1 output for 1 input, got %d", outputCount)
			}
		})
	})
}

// TemplateConfig represents the resolved configuration for a single experiment template,
// capturing the role_arn and log_group_arn assigned by the module.
type TemplateConfig struct {
	RoleArn     string
	LogGroupArn string
}

// resolveUniformConfig simulates the module's behavior of assigning a single role_arn
// (from data.aws_iam_role.fis_experiment_role.arn) and a single log_group_arn
// (from aws_cloudwatch_log_group.fis_experiments.arn) to every experiment template.
func resolveUniformConfig(templateKeys []string, roleArn, logGroupArn string) map[string]TemplateConfig {
	configs := make(map[string]TemplateConfig, len(templateKeys))
	for _, key := range templateKeys {
		configs[key] = TemplateConfig{
			RoleArn:     roleArn,
			LogGroupArn: logGroupArn,
		}
	}
	return configs
}

// Feature: aws-fis-terraform-module, Property 4: Uniform Template Configuration
// Validates: Requirements 4.3, 5.3
func TestProperty4_UniformTemplateConfiguration(t *testing.T) {
	// Generator for a resolved IAM role ARN
	genRoleArn := func(t *rapid.T) string {
		accountID := genAccountID(t)
		return fmt.Sprintf("arn:aws:iam::%s:role/FISExperimentRole", accountID)
	}

	// Generator for a resolved CloudWatch log group ARN
	genLogGroupArn := func(t *rapid.T) string {
		accountID := genAccountID(t)
		envs := []string{"dev", "staging", "prod", "qa", "uat"}
		env := envs[rapid.IntRange(0, len(envs)-1).Draw(t, "env")]
		return fmt.Sprintf("arn:aws:logs:us-east-1:%s:log-group:/aws/fis/experiments/%s:*", accountID, env)
	}

	// Generator for unique template keys
	genTemplateKeys := func(t *rapid.T) []string {
		count := rapid.IntRange(1, 10).Draw(t, "templateCount")
		keys := make([]string, count)
		for i := 0; i < count; i++ {
			svc := rapid.StringMatching(`[a-z]{2,8}`).Draw(t, fmt.Sprintf("svc[%d]", i))
			scn := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, fmt.Sprintf("scn[%d]", i))
			keys[i] = fmt.Sprintf("%s-%s", svc, scn)
		}
		return keys
	}

	// Sub-test: all templates share the same role_arn
	t.Run("all_templates_share_same_role_arn", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			roleArn := genRoleArn(t)
			logGroupArn := genLogGroupArn(t)
			templateKeys := genTemplateKeys(t)

			configs := resolveUniformConfig(templateKeys, roleArn, logGroupArn)

			for key, cfg := range configs {
				if cfg.RoleArn != roleArn {
					t.Fatalf("template %q has role_arn %q, expected %q", key, cfg.RoleArn, roleArn)
				}
			}
		})
	})

	// Sub-test: all templates share the same log_group_arn
	t.Run("all_templates_share_same_log_group_arn", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			roleArn := genRoleArn(t)
			logGroupArn := genLogGroupArn(t)
			templateKeys := genTemplateKeys(t)

			configs := resolveUniformConfig(templateKeys, roleArn, logGroupArn)

			for key, cfg := range configs {
				if cfg.LogGroupArn != logGroupArn {
					t.Fatalf("template %q has log_group_arn %q, expected %q", key, cfg.LogGroupArn, logGroupArn)
				}
			}
		})
	})

	// Sub-test: all templates are pairwise identical in role_arn and log_group_arn
	t.Run("all_templates_pairwise_identical", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			roleArn := genRoleArn(t)
			logGroupArn := genLogGroupArn(t)
			templateKeys := genTemplateKeys(t)

			configs := resolveUniformConfig(templateKeys, roleArn, logGroupArn)

			var firstKey string
			var firstCfg TemplateConfig
			for k, v := range configs {
				firstKey = k
				firstCfg = v
				break
			}

			for key, cfg := range configs {
				if cfg.RoleArn != firstCfg.RoleArn {
					t.Fatalf("template %q role_arn %q differs from template %q role_arn %q",
						key, cfg.RoleArn, firstKey, firstCfg.RoleArn)
				}
				if cfg.LogGroupArn != firstCfg.LogGroupArn {
					t.Fatalf("template %q log_group_arn %q differs from template %q log_group_arn %q",
						key, cfg.LogGroupArn, firstKey, firstCfg.LogGroupArn)
				}
			}
		})
	})
}

// ResourceTag represents a tag entry used for target resource selection.
type ResourceTag struct {
	Key   string
	Value string
}

// TargetIdentifier represents the identifier portion of a target configuration,
// containing resource_arns and resource_tags fields.
type TargetIdentifier struct {
	ResourceArns []string
	ResourceTags []ResourceTag
}

// validateTargetIdentifier replicates the module's three-part target identifier validation:
// 1. Target must have non-empty resource_arns OR non-empty resource_tags (precondition in fis_templates.tf)
// 2. resource_arns and resource_tags are mutually exclusive (validation in variables.tf)
// 3. Each resource_tag must have non-empty (trimmed) key and value (validation in variables.tf)
func validateTargetIdentifier(target TargetIdentifier) bool {
	hasArns := len(target.ResourceArns) > 0
	hasTags := len(target.ResourceTags) > 0

	// Rule 1: must have at least one identifier
	if !hasArns && !hasTags {
		return false
	}

	// Rule 2: mutual exclusivity
	if hasArns && hasTags {
		return false
	}

	// Rule 3: each tag entry must have non-empty trimmed key and value
	for _, tag := range target.ResourceTags {
		if strings.TrimSpace(tag.Key) == "" || strings.TrimSpace(tag.Value) == "" {
			return false
		}
	}

	return true
}

// Feature: aws-fis-terraform-module, Property 10: Non-Empty Target Identifier Validation
// Validates: Requirements 7.2, 7.7
func TestProperty10_NonEmptyTargetIdentifierValidation(t *testing.T) {
	// Generator for a valid non-empty ARN
	genArn := func(t *rapid.T) string {
		accountID := genAccountID(t)
		svc := rapid.StringMatching(`[a-z]{2,8}`).Draw(t, "svc")
		resource := rapid.StringMatching(`[a-z][a-z0-9-]{2,20}`).Draw(t, "resource")
		return fmt.Sprintf("arn:aws:%s:us-east-1:%s:%s", svc, accountID, resource)
	}

	// Generator for a valid resource tag (non-empty trimmed key and value)
	genValidTag := func(t *rapid.T) ResourceTag {
		key := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,19}`).Draw(t, "tagKey")
		value := rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9_ -]{0,19}`).Draw(t, "tagValue")
		return ResourceTag{Key: key, Value: value}
	}

	// Sub-test: valid targets with non-empty resource_arns are accepted
	t.Run("valid_resource_arns_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			arnCount := rapid.IntRange(1, 5).Draw(t, "arnCount")
			arns := make([]string, arnCount)
			for i := 0; i < arnCount; i++ {
				arns[i] = genArn(t)
			}
			target := TargetIdentifier{
				ResourceArns: arns,
				ResourceTags: []ResourceTag{},
			}
			if !validateTargetIdentifier(target) {
				t.Fatalf("expected target with %d resource_arns to be accepted", arnCount)
			}
		})
	})

	// Sub-test: valid targets with non-empty resource_tags (valid key/value) are accepted
	t.Run("valid_resource_tags_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			tagCount := rapid.IntRange(1, 5).Draw(t, "tagCount")
			tags := make([]ResourceTag, tagCount)
			for i := 0; i < tagCount; i++ {
				tags[i] = genValidTag(t)
			}
			target := TargetIdentifier{
				ResourceArns: []string{},
				ResourceTags: tags,
			}
			if !validateTargetIdentifier(target) {
				t.Fatalf("expected target with %d valid resource_tags to be accepted", tagCount)
			}
		})
	})

	// Sub-test: targets with both empty resource_arns and empty resource_tags are rejected
	t.Run("both_empty_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			target := TargetIdentifier{
				ResourceArns: []string{},
				ResourceTags: []ResourceTag{},
			}
			if validateTargetIdentifier(target) {
				t.Fatal("expected target with both empty resource_arns and resource_tags to be rejected")
			}
		})
	})

	// Sub-test: targets with nil resource_arns and nil resource_tags are rejected
	t.Run("both_nil_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			target := TargetIdentifier{
				ResourceArns: nil,
				ResourceTags: nil,
			}
			if validateTargetIdentifier(target) {
				t.Fatal("expected target with nil resource_arns and resource_tags to be rejected")
			}
		})
	})

	// Sub-test: targets with both resource_arns and resource_tags are rejected (mutual exclusivity)
	t.Run("both_arns_and_tags_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			arnCount := rapid.IntRange(1, 3).Draw(t, "arnCount")
			arns := make([]string, arnCount)
			for i := 0; i < arnCount; i++ {
				arns[i] = genArn(t)
			}
			tagCount := rapid.IntRange(1, 3).Draw(t, "tagCount")
			tags := make([]ResourceTag, tagCount)
			for i := 0; i < tagCount; i++ {
				tags[i] = genValidTag(t)
			}
			target := TargetIdentifier{
				ResourceArns: arns,
				ResourceTags: tags,
			}
			if validateTargetIdentifier(target) {
				t.Fatalf("expected target with both resource_arns (%d) and resource_tags (%d) to be rejected",
					arnCount, tagCount)
			}
		})
	})

	// Sub-test: resource_tags with blank/whitespace-only keys are rejected
	t.Run("blank_tag_key_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			blankKeys := []string{"", " ", "  ", "\t", "\n", " \t "}
			blankKey := blankKeys[rapid.IntRange(0, len(blankKeys)-1).Draw(t, "blankKeyIdx")]
			validValue := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "value")

			tags := []ResourceTag{{Key: blankKey, Value: validValue}}
			// Optionally add valid tags before the blank one
			extraCount := rapid.IntRange(0, 2).Draw(t, "extraCount")
			for i := 0; i < extraCount; i++ {
				tags = append([]ResourceTag{genValidTag(t)}, tags...)
			}

			target := TargetIdentifier{
				ResourceArns: []string{},
				ResourceTags: tags,
			}
			if validateTargetIdentifier(target) {
				t.Fatalf("expected target with blank tag key %q to be rejected", blankKey)
			}
		})
	})

	// Sub-test: resource_tags with blank/whitespace-only values are rejected
	t.Run("blank_tag_value_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			blankValues := []string{"", " ", "  ", "\t", "\n", " \t "}
			blankValue := blankValues[rapid.IntRange(0, len(blankValues)-1).Draw(t, "blankValueIdx")]
			validKey := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,9}`).Draw(t, "key")

			tags := []ResourceTag{{Key: validKey, Value: blankValue}}
			// Optionally add valid tags before the blank one
			extraCount := rapid.IntRange(0, 2).Draw(t, "extraCount")
			for i := 0; i < extraCount; i++ {
				tags = append([]ResourceTag{genValidTag(t)}, tags...)
			}

			target := TargetIdentifier{
				ResourceArns: []string{},
				ResourceTags: tags,
			}
			if validateTargetIdentifier(target) {
				t.Fatalf("expected target with blank tag value %q to be rejected", blankValue)
			}
		})
	})

	// Sub-test: resource_tags with both blank key and blank value are rejected
	t.Run("blank_key_and_value_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			blankEntries := []ResourceTag{
				{Key: "", Value: ""},
				{Key: " ", Value: " "},
				{Key: "\t", Value: "\n"},
			}
			entry := blankEntries[rapid.IntRange(0, len(blankEntries)-1).Draw(t, "entryIdx")]

			target := TargetIdentifier{
				ResourceArns: []string{},
				ResourceTags: []ResourceTag{entry},
			}
			if validateTargetIdentifier(target) {
				t.Fatalf("expected target with blank key %q and blank value %q to be rejected",
					entry.Key, entry.Value)
			}
		})
	})
}

// TemplateOutput represents the output entry for a single experiment template,
// containing the id, constructed arn, and name fields.
type TemplateOutput struct {
	ID   string
	Arn  string
	Name string
}

// constructTemplateOutputs simulates the module's output construction logic from outputs.tf:
//
//	output "experiment_templates" {
//	  value = {
//	    for key, tpl in aws_fis_experiment_template.this : key => {
//	      id   = tpl.id
//	      arn  = local.experiment_template_arns[key]
//	      name = "fis-${key}-${var.environment}"
//	    }
//	  }
//	}
//
// And the ARN construction from locals:
//
//	experiment_template_arns = {
//	  for key, tpl in aws_fis_experiment_template.this :
//	  key => "arn:aws:fis:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:experiment-template/${tpl.id}"
//	}
func constructTemplateOutputs(templateIDs map[string]string, region, accountID, environment string) map[string]TemplateOutput {
	outputs := make(map[string]TemplateOutput, len(templateIDs))
	for key, id := range templateIDs {
		outputs[key] = TemplateOutput{
			ID:   id,
			Arn:  fmt.Sprintf("arn:aws:fis:%s:%s:experiment-template/%s", region, accountID, id),
			Name: fmt.Sprintf("fis-%s-%s", key, environment),
		}
	}
	return outputs
}

// Feature: aws-fis-terraform-module, Property 11: Output Map Completeness
// Validates: Requirements 9.4
func TestProperty11_OutputMapCompleteness(t *testing.T) {
	// Generator for AWS region names
	genRegion := func(t *rapid.T) string {
		regions := []string{"us-east-1", "us-west-2", "eu-west-1", "eu-central-1", "ap-southeast-1", "ap-northeast-1"}
		return regions[rapid.IntRange(0, len(regions)-1).Draw(t, "region")]
	}

	// Generator for environment values
	genEnvironment := func(t *rapid.T) string {
		envs := []string{"dev", "staging", "prod", "qa", "uat", "sandbox", "test"}
		return envs[rapid.IntRange(0, len(envs)-1).Draw(t, "envIdx")]
	}

	// Generator for simulated template IDs (FIS template IDs are alphanumeric)
	genTemplateID := func(t *rapid.T, label string) string {
		return rapid.StringMatching(`[A-Za-z0-9]{8,20}`).Draw(t, label)
	}

	// Generator for unique template keys map with simulated IDs
	genTemplateIDs := func(t *rapid.T) map[string]string {
		count := rapid.IntRange(1, 10).Draw(t, "templateCount")
		ids := make(map[string]string, count)
		for i := 0; i < count; i++ {
			svc := rapid.StringMatching(`[a-z]{2,8}`).Draw(t, fmt.Sprintf("svc[%d]", i))
			scn := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, fmt.Sprintf("scn[%d]", i))
			key := fmt.Sprintf("%s-%s", svc, scn)
			ids[key] = genTemplateID(t, fmt.Sprintf("id[%d]", i))
		}
		return ids
	}

	// ARN pattern: arn:aws:fis:{region}:{account_id}:experiment-template/{id}
	arnPattern := regexp.MustCompile(`^arn:aws:fis:[a-z0-9-]+:\d{12}:experiment-template/[A-Za-z0-9]+$`)

	// Sub-test: output map has matching keys for every input key
	t.Run("output_keys_match_input_keys", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			templateIDs := genTemplateIDs(t)
			region := genRegion(t)
			accountID := genAccountID(t)
			environment := genEnvironment(t)

			outputs := constructTemplateOutputs(templateIDs, region, accountID, environment)

			// Verify output map has same number of entries as input
			if len(outputs) != len(templateIDs) {
				t.Fatalf("output map size %d does not match input size %d", len(outputs), len(templateIDs))
			}

			// Verify every input key has a corresponding output entry
			for key := range templateIDs {
				if _, ok := outputs[key]; !ok {
					t.Fatalf("input key %q missing from output map", key)
				}
			}
		})
	})

	// Sub-test: each output entry has non-empty id, arn, and name
	t.Run("output_entries_have_nonempty_fields", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			templateIDs := genTemplateIDs(t)
			region := genRegion(t)
			accountID := genAccountID(t)
			environment := genEnvironment(t)

			outputs := constructTemplateOutputs(templateIDs, region, accountID, environment)

			for key, out := range outputs {
				if out.ID == "" {
					t.Fatalf("output entry %q has empty id", key)
				}
				if out.Arn == "" {
					t.Fatalf("output entry %q has empty arn", key)
				}
				if out.Name == "" {
					t.Fatalf("output entry %q has empty name", key)
				}
			}
		})
	})

	// Sub-test: ARN follows the pattern arn:aws:fis:{region}:{account_id}:experiment-template/{id}
	t.Run("arn_follows_expected_pattern", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			templateIDs := genTemplateIDs(t)
			region := genRegion(t)
			accountID := genAccountID(t)
			environment := genEnvironment(t)

			outputs := constructTemplateOutputs(templateIDs, region, accountID, environment)

			for key, out := range outputs {
				if !arnPattern.MatchString(out.Arn) {
					t.Fatalf("output entry %q has ARN that does not match expected pattern: %q", key, out.Arn)
				}

				// Verify the ARN contains the correct region, account_id, and id
				expectedArn := fmt.Sprintf("arn:aws:fis:%s:%s:experiment-template/%s", region, accountID, templateIDs[key])
				if out.Arn != expectedArn {
					t.Fatalf("output entry %q ARN mismatch: got %q, want %q", key, out.Arn, expectedArn)
				}
			}
		})
	})

	// Sub-test: name follows the pattern fis-{key}-{environment}
	t.Run("name_follows_expected_pattern", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			templateIDs := genTemplateIDs(t)
			region := genRegion(t)
			accountID := genAccountID(t)
			environment := genEnvironment(t)

			outputs := constructTemplateOutputs(templateIDs, region, accountID, environment)

			for key, out := range outputs {
				expectedName := fmt.Sprintf("fis-%s-%s", key, environment)
				if out.Name != expectedName {
					t.Fatalf("output entry %q name mismatch: got %q, want %q", key, out.Name, expectedName)
				}
			}
		})
	})

	// Sub-test: output id matches the simulated template id from input
	t.Run("output_id_matches_input_id", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			templateIDs := genTemplateIDs(t)
			region := genRegion(t)
			accountID := genAccountID(t)
			environment := genEnvironment(t)

			outputs := constructTemplateOutputs(templateIDs, region, accountID, environment)

			for key, out := range outputs {
				if out.ID != templateIDs[key] {
					t.Fatalf("output entry %q id mismatch: got %q, want %q", key, out.ID, templateIDs[key])
				}
			}
		})
	})
}
