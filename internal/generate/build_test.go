package generate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func protoRPCOpts() BuildOptions {
	return BuildOptions{Stages: []string{StageProto, StageRPC}}
}

func TestBuildWritesLayersFromProto(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "pax-utah-cfg", "charlemagne.json")
	if err := os.MkdirAll(filepath.Dir(catalog), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog, []byte(`{
		"services": [{
			"name": "users",
			"id": "users",
			"endpoints": {"grpc": [{"name": "create"}, {"name": "GetUserByID"}]}
		}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(root, "services", "users")
	if err := os.MkdirAll(filepath.Join(dest, "internal", "usecase"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "go.mod"), []byte("module github.com/benthinnick/users\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := Service{
		Name: "users",
		ID:   "users",
		Endpoints: Endpoints{GRPC: []Endpoint{
			{Name: "create"},
			{Name: "GetUserByID"},
		}},
	}
	if err := Build(dest, svc, catalog, protoRPCOpts()); err != nil {
		t.Fatal(err)
	}

	protoBody, err := os.ReadFile(filepath.Join(root, "pax-utah-cfg", "proto", "services", "users", "users.proto"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(protoBody), "rpc Create(") {
		t.Fatalf("proto stubs:\n%s", protoBody)
	}

	checks := map[string][]string{
		filepath.Join("internal", "entity", "create.gen.go"): {
			"type CreateRequest struct",
			"func CreateRequestFromProto",
			"func (m CreateRequest) ToProto()",
		},
		filepath.Join("internal", "entity", "get_user_by_id.gen.go"): {
			"type GetUserByIDRequest struct",
			"GetUserByIDRequestFromProto",
		},
		filepath.Join("internal", "usecase", "rpc.gen.go"): {
			"Create(ctx context.Context, in entity.CreateRequest)",
			"GetUserByID(ctx context.Context, in entity.GetUserByIDRequest)",
			"type UnimplementedRPC struct",
		},
		filepath.Join("internal", "usecase", "create.go"): {
			"func (u *useCase) Create(",
		},
		filepath.Join("internal", "usecase", "get_user_by_id.go"): {
			"func (u *useCase) GetUserByID(",
		},
		filepath.Join("internal", "server", "grpc.gen.go"): {
			"RegisterUsersServer",
			"entity.CreateRequestFromProto",
			"codes.NotFound",
		},
	}
	for rel, needles := range checks {
		body, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		got := string(body)
		for _, n := range needles {
			if !strings.Contains(got, n) {
				t.Errorf("%s missing %q\n%s", rel, n, got)
			}
		}
	}

	if err := Build(dest, svc, catalog, protoRPCOpts()); err != nil {
		t.Fatal(err)
	}
	stub, err := os.ReadFile(filepath.Join(dest, "internal", "usecase", "create.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stub), "Code generated") {
		t.Fatal("usecase stub should not be marked generated")
	}
}

func TestBuildRequiresExistingService(t *testing.T) {
	dir := t.TempDir()
	catalog := filepath.Join(dir, "charlemagne.json")
	if err := os.WriteFile(catalog, []byte(`{"services":[{"name":"users","id":"users"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Build(filepath.Join(dir, "users"), Service{Name: "users", ID: "users"}, catalog, protoRPCOpts())
	if err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("got %v", err)
	}
}

func TestBuildCodegenIncludesProtoFields(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "charlemagne.json")
	if err := os.WriteFile(catalog, []byte(`{"services":[{"name":"users","id":"users","endpoints":{"grpc":[{"name":"GetUserByID"}]}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "users")
	if err := os.MkdirAll(filepath.Join(dest, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "go.mod"), []byte("module github.com/benthinnick/users\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := Service{Name: "users", ID: "users", Endpoints: Endpoints{GRPC: []Endpoint{{Name: "GetUserByID"}}}}
	if err := Build(dest, svc, catalog, protoRPCOpts()); err != nil {
		t.Fatal(err)
	}
	protoPath := filepath.Join(root, "proto", "services", "users", "users.proto")
	body, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(body), "message GetUserByIDRequest {}", "message GetUserByIDRequest {\n  string id = 1;\n}", 1)
	if err := os.WriteFile(protoPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(dest, svc, catalog, protoRPCOpts()); err != nil {
		t.Fatal(err)
	}
	ent, err := os.ReadFile(filepath.Join(dest, "internal", "entity", "get_user_by_id.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(ent)
	if !strings.Contains(got, "Id string `json:\"id\"`") {
		t.Fatalf("expected id field:\n%s", got)
	}
	if !strings.Contains(got, "in.GetId()") {
		t.Fatalf("expected getter:\n%s", got)
	}
}

func TestBuildStageProtoOnly(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "charlemagne.json")
	if err := os.WriteFile(catalog, []byte(`{"services":[{"name":"users","id":"users","endpoints":{"grpc":[{"name":"create"}]}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "users")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "go.mod"), []byte("module github.com/benthinnick/users\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{Name: "users", ID: "users", Endpoints: Endpoints{GRPC: []Endpoint{{Name: "create"}}}}
	var log bytes.Buffer
	if err := Build(dest, svc, catalog, BuildOptions{Stage: StageProto, Log: &log}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "==> proto") {
		t.Fatalf("log: %s", log.String())
	}
	if _, err := os.Stat(filepath.Join(root, "proto", "services", "users", "users.proto")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "internal", "server", "grpc.gen.go")); err == nil {
		t.Fatal("rpc stage should not have run")
	}
}

func TestBuildORMSkipsWithoutSchema(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "go.mod"), []byte("module github.com/benthinnick/users\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := filepath.Join(t.TempDir(), "charlemagne.json")
	if err := os.WriteFile(catalog, []byte(`{"services":[{"name":"users","id":"users","dependencies":["postgresql"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{Name: "users", ID: "users", Dependencies: []string{"postgresql"}}
	var log bytes.Buffer
	if err := Build(dest, svc, catalog, BuildOptions{Stage: StageORM, Log: &log}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "no schema files") {
		t.Fatalf("log: %s", log.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "internal", "ent", "generate.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "internal", "ent", "schema", "README.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "internal", "providers", "ent.go")); err == nil {
		t.Fatal("should not wire ent without schemas")
	}
}

func TestSelectStages(t *testing.T) {
	got, err := selectStages(BuildOptions{From: StageRPC})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{StageRPC, StageORM, StageTidy}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
	if _, err := selectStages(BuildOptions{Stage: "nope"}); err == nil {
		t.Fatal("expected unknown stage")
	}
	if _, err := selectStages(BuildOptions{Stage: StageProto, From: StageRPC}); err == nil {
		t.Fatal("expected both-flags error")
	}
}
