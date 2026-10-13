// Package generate writes a resttest-shaped Go service from embedded templates.
package generate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/benthinnick/charlemagne/templates"
)

const healthRoot = "health"

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Project is the template data for one generated service.
type Project struct {
	Name             string
	ID               string
	Module           string
	ServiceName      string
	PostgresUser     string
	PostgresDB       string
	PostgresPassword string
	HasPostgres      bool
	HasVault         bool
}

// Generate writes the health-service tree into dest and runs go mod tidy plus
// go generate so go.sum and mockgen output exist.
func Generate(dest string, proj Project) error {
	if err := ensureEmptyDest(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if err := renderTree(dest, proj); err != nil {
		return err
	}
	if err := runGo(dest, "mod", "tidy"); err != nil {
		return err
	}
	if proj.HasPostgres {
		if err := runGo(dest, "generate", "./internal/providers"); err != nil {
			return err
		}
	}
	return nil
}

func ensureEmptyDest(dest string) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("destination %s is not empty", dest)
	}
	return nil
}

func renderTree(dest string, proj Project) error {
	return fs.WalkDir(templates.HealthFS, healthRoot, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel := strings.TrimPrefix(name, healthRoot+"/")
		rel = filepath.FromSlash(rel)
		rel = strings.TrimSuffix(rel, ".tmpl")
		if skipTemplate(rel, proj) {
			return nil
		}

		body, err := templates.HealthFS.ReadFile(name)
		if err != nil {
			return err
		}

		tmpl, err := template.New(filepath.ToSlash(rel)).Parse(string(body))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", name, err)
		}

		var out bytes.Buffer
		if err := tmpl.Execute(&out, proj); err != nil {
			return fmt.Errorf("execute template %s: %w", name, err)
		}

		target := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		return os.WriteFile(target, out.Bytes(), mode)
	})
}

func skipTemplate(rel string, proj Project) bool {
	slash := filepath.ToSlash(rel)
	if !proj.HasVault {
		switch slash {
		case "internal/config/vault.go", "deploy/vault/seed.sh", "deploy/k8s/03-jobs.yaml":
			return true
		}
	}
	if !proj.HasPostgres {
		switch slash {
		case "tests/mocks/providers/mocks.go",
			"internal/ent/generate.go",
			"internal/ent/schema/README.md":
			return true
		}
	}
	return false
}

func runGo(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOPRIVATE=github.com/benthinnick/distlibs",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
