package helper

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ============================================================================
// Governance and Layout Validation Tests
// Validates that all required governance scripts, docs, and the proto snapshot
// exist and follow the service-scoped SDK conventions.
// ============================================================================

func TestProtoSnapshotExistsAndIsFile(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	protoFile := filepath.Join(filepath.Dir(filepath.Dir(thisFile)), "proto", "s3minio.proto")

	info, err := os.Stat(protoFile)
	if err != nil {
		t.Fatalf("proto snapshot not found at %s: %v", protoFile, err)
	}
	if info.IsDir() {
		t.Fatalf("proto snapshot path is a directory, not a file: %s", protoFile)
	}
}

func TestRequiredGovernanceDocsExist(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))))

	requiredDocs := []string{
		filepath.Join(rootDir, "docs", "sdk", "architecture.md"),
		filepath.Join(rootDir, "docs", "sdk", "proto-lifecycle.md"),
		filepath.Join(rootDir, "docs", "sdk", "grpc-stub-maintenance.md"),
		filepath.Join(rootDir, "docs", "sdk", "versioning-policy.md"),
		filepath.Join(rootDir, "docs", "sdk", "service-onboarding.md"),
		filepath.Join(rootDir, "docs", "sdk", "change-log-template.md"),
		filepath.Join(rootDir, "docs", "sdk", "stub-change-log.md"),
		filepath.Join(rootDir, "docs", "sdk", "multi-service-layout.md"),
	}

	for _, doc := range requiredDocs {
		info, err := os.Stat(doc)
		if err != nil {
			t.Errorf("required governance doc missing: %s (error: %v)", doc, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("expected file but found directory: %s", doc)
		}
	}
}

func TestRequiredGovernanceScriptsExist(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))))

	requiredScripts := []string{
		filepath.Join(rootDir, "scripts", "proto", "update-s3minio-proto.sh"),
		filepath.Join(rootDir, "scripts", "proto", "gen-s3minio-client.sh"),
		filepath.Join(rootDir, "scripts", "proto", "check-stub-drift.sh"),
		filepath.Join(rootDir, "scripts", "proto", "new-service-scaffold.sh"),
		filepath.Join(rootDir, "scripts", "check-no-direct-s3minio-http.sh"),
	}

	for _, script := range requiredScripts {
		info, err := os.Stat(script)
		if err != nil {
			t.Errorf("required script missing: %s (error: %v)", script, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("expected file but found directory: %s", script)
			continue
		}
		// Verify file is executable (check user execute bit)
		if info.Mode()&0100 == 0 {
			t.Errorf("script is not executable: %s", script)
		}
	}
}

func TestBufConfigsExist(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))))

	requiredConfigs := []string{
		filepath.Join(rootDir, "buf.yaml"),
		filepath.Join(rootDir, "buf.gen.yaml"),
	}

	for _, config := range requiredConfigs {
		info, err := os.Stat(config)
		if err != nil {
			t.Errorf("required buf config missing: %s (error: %v)", config, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("expected file but found directory: %s", config)
		}
	}
}
