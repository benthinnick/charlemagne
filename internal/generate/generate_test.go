package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRefusesNonEmptyDest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, err := ProjectFromService(Service{Name: "demo", ID: "demo", Dependencies: []string{"postgresql", "vault"}})
	if err != nil {
		t.Fatal(err)
	}
	err = Generate(dir, proj)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("got %v, want not empty", err)
	}
}

func TestGenerateHealthService(t *testing.T) {
	if testing.Short() {
		t.Skip("skips module download and generated go test")
	}

	root := t.TempDir()
	dest := filepath.Join(root, "demo")

	proj, err := ProjectFromService(Service{
		Name: "demo", ID: "demo-id", Dependencies: []string{"postgresql", "vault"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(dest, proj); err != nil {
		t.Fatal(err)
	}

	wantFiles := []string{
		"go.mod",
		"go.sum",
		"Makefile",
		"cmd/api/main.go",
		"internal/config/config.go",
		"internal/config/vault.go",
		"internal/entity/response.go",
		"internal/providers/providers.go",
		"internal/ent/generate.go",
		"internal/ent/schema/README.md",
		"internal/server/server.go",
		"internal/server/grpc.gen.go",
		"internal/usecase/readiness.go",
		"internal/usecase/rpc.gen.go",
		"tests/usecase/readiness_test.go",
		"tests/mocks/providers/mocks.go",
		"build/Dockerfile",
		"deploy/docker-compose.yml",
		"deploy/vault/seed.sh",
		"deploy/k8s/03-jobs.yaml",
		"deploy/k8s/04-api.yaml",
	}
	for _, rel := range wantFiles {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}

	mod, err := os.ReadFile(filepath.Join(dest, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mod), "module github.com/benthinnick/demo") {
		t.Fatalf("go.mod missing module path:\n%s", mod)
	}
	if !strings.Contains(string(mod), "github.com/benthinnick/distlibs") {
		t.Fatalf("go.mod missing distlibs:\n%s", mod)
	}

	cfg, err := os.ReadFile(filepath.Join(dest, "internal", "config", "config.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), `envDefault:"demo-id"`) {
		t.Fatalf("SERVICE_ID default missing:\n%s", cfg)
	}

	needles := []string{"S3_", "/uploads", "minio", "{{"}
	err = filepath.WalkDir(dest, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(body)
		for _, needle := range needles {
			if strings.Contains(strings.ToLower(text), strings.ToLower(needle)) {
				t.Errorf("%s contains %q", path, needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	testGenerated(t, dest)
}

func TestGenerateWithoutDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skips module download and generated go test")
	}

	dest := filepath.Join(t.TempDir(), "lite")
	proj, err := ProjectFromService(Service{Name: "lite", ID: "lite-id"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(dest, proj); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{
		"internal/config/vault.go",
		"deploy/vault/seed.sh",
		"deploy/k8s/03-jobs.yaml",
		"tests/mocks/providers/mocks.go",
		"internal/ent/generate.go",
		"internal/ent/schema/README.md",
	} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("did not expect %s", rel)
		}
	}

	err = filepath.WalkDir(dest, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(body)
		if strings.Contains(text, "{{") {
			t.Errorf("%s contains leftover template action", path)
		}
		if strings.Contains(text, "POSTGRES_") || strings.Contains(text, "VAULT_") {
			t.Errorf("%s still mentions postgres/vault env: %s", path, text[:min(120, len(text))])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	testGenerated(t, dest)
}

func testGenerated(t *testing.T, dest string) {
	t.Helper()
	testCmd := exec.Command("go", "test", "./...", "-count=1")
	testCmd.Dir = dest
	if out, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated go test: %v\n%s", err, out)
	}

	vetCmd := exec.Command("go", "vet", "./...")
	vetCmd.Dir = dest
	if out, err := vetCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated go vet: %v\n%s", err, out)
	}
}
