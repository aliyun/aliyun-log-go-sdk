// Package testutil provides shared helpers used by both the in-package
// unit tests (default build) and the e2e tests gated behind the `e2e`
// build tag.
//
// This package intentionally does NOT import the root sls package so it
// can be consumed from `package sls` test files without creating an
// import cycle. Helpers that need to construct a *sls.Client live in the
// sub-package internal/testutil/clienthelper and must be consumed from
// external test packages (`package sls_test`).
package testutil

import (
	"os"
	"testing"
)

// E2EConfig captures the LOG_TEST_* environment variables an e2e test
// typically needs. Optional fields may be empty when the corresponding
// env var is not set.
type E2EConfig struct {
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	Project         string
	Logstore        string
	Region          string
	RoleArn         string
	CmkID           string
	CmkEndpoint     string
}

// Env var names. Kept as exported constants so test files can reference
// them in skip messages or failure assertions.
const (
	EnvEndpoint        = "LOG_TEST_ENDPOINT"
	EnvAccessKeyID     = "LOG_TEST_ACCESS_KEY_ID"
	EnvAccessKeySecret = "LOG_TEST_ACCESS_KEY_SECRET"
	EnvProject         = "LOG_TEST_PROJECT"
	EnvLogstore        = "LOG_TEST_LOGSTORE"
	EnvRegion          = "LOG_TEST_REGION"
	EnvRoleArn         = "LOG_TEST_ROLE_ARN"
	EnvCmkID           = "LOG_TEST_CMK_ID"
	EnvCmkEndpoint     = "LOG_TEST_CMK_ENDPOINT"
)

// requiredEnv lists the env vars that MUST be set for any e2e test
// against a real SLS endpoint.
var requiredEnv = []string{
	EnvEndpoint,
	EnvAccessKeyID,
	EnvAccessKeySecret,
	EnvProject,
}

// RequireE2E skips the test unless every required LOG_TEST_* env var is
// populated. Call it as the first line of any e2e test (or its
// SetupSuite) so the suite is silently skipped on machines / CI runs
// without credentials configured.
func RequireE2E(t *testing.T) {
	t.Helper()
	for _, name := range requiredEnv {
		if os.Getenv(name) == "" {
			t.Skipf("SKIP: requires LOG_TEST_* env (missing %s)", name)
		}
	}
}

// LoadE2EConfig reads the LOG_TEST_* env vars into an E2EConfig. The
// second return value is true when all required vars (endpoint, AK ID,
// AK secret, project) are non-empty.
func LoadE2EConfig() (E2EConfig, bool) {
	cfg := E2EConfig{
		Endpoint:        os.Getenv(EnvEndpoint),
		AccessKeyID:     os.Getenv(EnvAccessKeyID),
		AccessKeySecret: os.Getenv(EnvAccessKeySecret),
		Project:         os.Getenv(EnvProject),
		Logstore:        os.Getenv(EnvLogstore),
		Region:          os.Getenv(EnvRegion),
		RoleArn:         os.Getenv(EnvRoleArn),
		CmkID:           os.Getenv(EnvCmkID),
		CmkEndpoint:     os.Getenv(EnvCmkEndpoint),
	}
	ok := cfg.Endpoint != "" &&
		cfg.AccessKeyID != "" &&
		cfg.AccessKeySecret != "" &&
		cfg.Project != ""
	return cfg, ok
}
