package generate

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

const (
	entOpenBegin = "// charlemagne:ent-open"
	entOpenEnd   = "// charlemagne:ent-open-end"
)

func runORM(dest string, proj Project, opts BuildOptions) error {
	if !proj.HasPostgres {
		logStage(opts, "orm (skipped: no postgresql)")
		return nil
	}
	if err := ensureEntScaffold(dest); err != nil {
		return err
	}
	if !hasEntSchemas(dest) {
		logStage(opts, "orm (no schema files; tables are owned by the service)")
		return nil
	}
	logStage(opts, "orm")
	if err := runGo(dest, "generate", "./internal/ent"); err != nil {
		return err
	}
	return wireEnt(dest, proj)
}

func ensureEntScaffold(dest string) error {
	files := map[string]string{
		filepath.Join("internal", "ent", "generate.go"):         entGenerateSrc,
		filepath.Join("internal", "ent", "schema", "README.md"): entSchemaReadme,
	}
	for rel, body := range files {
		path := filepath.Join(dest, rel)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func hasEntSchemas(dest string) bool {
	dir := filepath.Join(dest, "internal", "ent", "schema")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if !ent.IsDir() && strings.HasSuffix(ent.Name(), ".go") {
			return true
		}
	}
	return false
}

func wireEnt(dest string, proj Project) error {
	if err := writeGo(filepath.Join(dest, "internal", "providers", "ent.go"), entOpenSrc(proj.Module)); err != nil {
		return err
	}
	if err := patchProvidersEnt(dest, proj.Module); err != nil {
		return err
	}
	return patchMainEnt(dest)
}

func patchProvidersEnt(dest, module string) error {
	path := filepath.Join(dest, "internal", "providers", "providers.go")
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := string(body)
	if strings.Contains(src, "Ent *ent.Client") {
		return nil
	}
	entImp := "\t\"" + module + "/internal/ent\"\n"
	if !strings.Contains(src, entImp) && strings.Contains(src, "\"github.com/jackc/pgx/v5/pgxpool\"") {
		src = strings.Replace(src, "\t\"github.com/jackc/pgx/v5/pgxpool\"\n", "\t\"github.com/jackc/pgx/v5/pgxpool\"\n"+entImp, 1)
	}
	src = strings.Replace(src, "type Providers struct {\n\tPostgres Postgres\n}", "type Providers struct {\n\tPostgres Postgres\n\tEnt      *ent.Client\n}", 1)
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return fmt.Errorf("gofmt providers.go: %w\n%s", err, src)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func patchMainEnt(dest string) error {
	path := filepath.Join(dest, "cmd", "api", "main.go")
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := string(body)
	if strings.Contains(src, entOpenBegin) {
		return nil
	}
	needle := "\tdeps.Postgres = providers.NewPostgres(pool)\n"
	if !strings.Contains(src, needle) {
		return fmt.Errorf("cmd/api/main.go: cannot find postgres wiring to attach ent")
	}
	block := needle + "\t" + entOpenBegin + "\n" +
		"\tentClient, err := providers.OpenEnt(ctx, cfg.Postgres.DSN)\n" +
		"\tif err != nil {\n" +
		"\t\treturn err\n" +
		"\t}\n" +
		"\tdefer entClient.Close()\n" +
		"\tdeps.Ent = entClient\n" +
		"\t" + entOpenEnd + "\n"
	src = strings.Replace(src, needle, block, 1)
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return fmt.Errorf("gofmt main.go: %w\n%s", err, src)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func entOpenSrc(module string) string {
	return `package providers

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"` + module + `/internal/ent"
)

// OpenEnt opens an ent client on the service Postgres DSN and applies schema
// from internal/ent/schema (tables defined in this service).
func OpenEnt(ctx context.Context, dsn string) (*ent.Client, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open ent driver: %w", err)
	}
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))
	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("migrate ent schema: %w", err)
	}
	return client, nil
}
`
}

const entGenerateSrc = `package ent

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema
`

const entSchemaReadme = `# Ent schemas

Table definitions live here and are owned by this service. Charlemagne does not
generate domain tables.

Add a Go file per entity, then run ` + "`charlemagne build <name>`" + ` (or ` + "`--stage orm`" + `):

` + "```go" + `
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").Unique(),
	}
}
` + "```" + `
`
