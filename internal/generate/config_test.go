package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCatalog(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "charlemagne.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadCatalogValid(t *testing.T) {
	path := writeCatalog(t, `{
		"services": [
			{"name": "demo", "id": "demo", "dependencies": ["postgresql", "vault"]},
			{"name": "lite", "id": "lite-1", "dependencies": []}
		],
		"gateways": []
	}`)
	cat, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := cat.ServiceByName("demo")
	if err != nil {
		t.Fatal(err)
	}
	if svc.ID != "demo" {
		t.Fatalf("id = %q", svc.ID)
	}
	if _, err := cat.ServiceByName("missing"); err == nil {
		t.Fatal("expected unknown service")
	}
}

func TestLoadCatalogRejects(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "duplicate names",
			body: `{"services":[{"name":"demo","id":"a"},{"name":"demo","id":"b"}]}`,
			want: "duplicate",
		},
		{
			name: "bad name",
			body: `{"services":[{"name":"Demo","id":"a"}]}`,
			want: "must match",
		},
		{
			name: "empty id",
			body: `{"services":[{"name":"demo","id":"  "}]}`,
			want: "id must not be empty",
		},
		{
			name: "unknown dependency",
			body: `{"services":[{"name":"demo","id":"a","dependencies":["redis"]}]}`,
			want: "unknown dependency",
		},
		{
			name: "bad grpc name",
			body: `{"services":[{"name":"demo","id":"a","endpoints":{"grpc":[{"name":"Get User"}]}}]}`,
			want: "proto identifier",
		},
		{
			name: "duplicate grpc",
			body: `{"services":[{"name":"demo","id":"a","endpoints":{"grpc":[{"name":"create"},{"name":"Create"}]}}]}`,
			want: "duplicate grpc",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadCatalog(writeCatalog(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestProjectFromServiceDeps(t *testing.T) {
	proj, err := ProjectFromService(Service{
		Name: "demo", ID: "svc-demo", Dependencies: []string{"postgres", "vault"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !proj.HasPostgres || !proj.HasVault || proj.ID != "svc-demo" {
		t.Fatalf("%+v", proj)
	}

	lite, err := ProjectFromService(Service{Name: "lite", ID: "lite"})
	if err != nil {
		t.Fatal(err)
	}
	if lite.HasPostgres || lite.HasVault {
		t.Fatalf("lite should have no deps: %+v", lite)
	}
}

func TestLoadCatalogGRPCEndpoints(t *testing.T) {
	path := writeCatalog(t, `{
		"services": [
			{
				"name": "users",
				"id": "users",
				"endpoints": {"grpc": [{"name": "create"}, {"name": "GetUserByID"}]}
			}
		]
	}`)
	cat, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := cat.ServiceByName("users")
	if err != nil {
		t.Fatal(err)
	}
	if len(svc.Endpoints.GRPC) != 2 || svc.Endpoints.GRPC[0].Name != "create" {
		t.Fatalf("%+v", svc.Endpoints)
	}
}

func TestResolveConfigPath(t *testing.T) {
	t.Setenv("CHARLEMAGNE_CONFIG", "")
	if got := ResolveConfigPath("/explicit.json"); got != "/explicit.json" {
		t.Fatalf("flag: %s", got)
	}
	t.Setenv("CHARLEMAGNE_CONFIG", "/from-env.json")
	if got := ResolveConfigPath(""); got != "/from-env.json" {
		t.Fatalf("env: %s", got)
	}
	t.Setenv("CHARLEMAGNE_CONFIG", "")
	if got := ResolveConfigPath(""); got != defaultConfig {
		t.Fatalf("default: %s", got)
	}
}
