package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPascalAndSnake(t *testing.T) {
	if got := pascalIdent("create"); got != "Create" {
		t.Fatalf("pascal create: %s", got)
	}
	if got := pascalIdent("GetUserByID"); got != "GetUserByID" {
		t.Fatalf("pascal GetUserByID: %s", got)
	}
	if got := snakeName("GetUserByID"); got != "get_user_by_id" {
		t.Fatalf("snake GetUserByID: %s", got)
	}
	if got := snakeName("Create"); got != "create" {
		t.Fatalf("snake Create: %s", got)
	}
}

func TestMergeProtoPreservesFields(t *testing.T) {
	dir := t.TempDir()
	catalog := filepath.Join(dir, "charlemagne.json")
	if err := os.WriteFile(catalog, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	protoDir := filepath.Join(dir, "proto", "services", "users")
	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `syntax = "proto3";

package users.v1;

option go_package = "github.com/benthinnick/users/internal/proto/services/users;usersv1";

service Users {
  rpc Create(CreateRequest) returns (CreateResponse);
}

message CreateRequest {
  string email = 1;
}
message CreateResponse {}
`
	path := filepath.Join(protoDir, "users.proto")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
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
	proj, err := ProjectFromService(svc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mergeProtoStubs(catalog, svc, proj); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "string email = 1;") {
		t.Fatalf("clobbered fields:\n%s", got)
	}
	if !strings.Contains(got, "rpc GetUserByID(GetUserByIDRequest) returns (GetUserByIDResponse);") {
		t.Fatalf("missing new rpc:\n%s", got)
	}
	if !strings.Contains(got, "message GetUserByIDRequest {}") {
		t.Fatalf("missing new message:\n%s", got)
	}
	if strings.Count(got, "rpc Create(") != 1 {
		t.Fatalf("duplicated Create:\n%s", got)
	}
}

func TestNewProtoFileFromCatalog(t *testing.T) {
	dir := t.TempDir()
	catalog := filepath.Join(dir, "charlemagne.json")
	if err := os.WriteFile(catalog, []byte(`{}`), 0o644); err != nil {
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
	proj, err := ProjectFromService(svc)
	if err != nil {
		t.Fatal(err)
	}
	path, err := mergeProtoStubs(catalog, svc, proj)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	for _, needle := range []string{
		"package users.v1;",
		"service Users {",
		"rpc Create(CreateRequest) returns (CreateResponse);",
		"rpc GetUserByID(GetUserByIDRequest) returns (GetUserByIDResponse);",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("missing %q in\n%s", needle, got)
		}
	}
}
