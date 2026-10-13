package generate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	depPostgresql = "postgresql"
	depVault      = "vault"
	defaultConfig = "../pax-utah-cfg/charlemagne.json"
)

// Catalog is the contents of charlemagne.json.
type Catalog struct {
	Services []Service `json:"services"`
	Gateways []any     `json:"gateways"`
}

// Service is one generatable API in the catalog.
type Service struct {
	Name         string    `json:"name"`
	ID           string    `json:"id"`
	Dependencies []string  `json:"dependencies"`
	Endpoints    Endpoints `json:"endpoints"`
}

// Endpoints lists transports and their RPCs. grpc names seed proto stubs.
type Endpoints struct {
	GRPC []Endpoint `json:"grpc"`
}

// Endpoint is one named RPC.
type Endpoint struct {
	Name string `json:"name"`
}

// ResolveConfigPath picks charlemagne.json: --config, CHARLEMAGNE_CONFIG, then
// ../pax-utah-cfg/charlemagne.json relative to the current directory.
func ResolveConfigPath(flagPath string) string {
	if strings.TrimSpace(flagPath) != "" {
		return flagPath
	}
	if env := strings.TrimSpace(os.Getenv("CHARLEMAGNE_CONFIG")); env != "" {
		return env
	}
	return defaultConfig
}

// LoadCatalog reads and validates a catalog file.
func LoadCatalog(path string) (*Catalog, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog %s: %w", path, err)
	}

	var cat Catalog
	if err := json.Unmarshal(body, &cat); err != nil {
		return nil, fmt.Errorf("parse catalog %s: %w", path, err)
	}
	if err := cat.validate(); err != nil {
		return nil, fmt.Errorf("catalog %s: %w", path, err)
	}
	return &cat, nil
}

// ServiceByName returns the named service or an error if it is missing.
func (c *Catalog) ServiceByName(name string) (Service, error) {
	for _, svc := range c.Services {
		if svc.Name == name {
			return svc, nil
		}
	}
	return Service{}, fmt.Errorf("unknown service %q", name)
}

func (c *Catalog) validate() error {
	seen := make(map[string]struct{}, len(c.Services))
	for i, svc := range c.Services {
		if !nameRE.MatchString(svc.Name) {
			return fmt.Errorf("services[%d].name %q must match %s", i, svc.Name, nameRE)
		}
		if strings.TrimSpace(svc.ID) == "" {
			return fmt.Errorf("services[%d] %q: id must not be empty", i, svc.Name)
		}
		if _, dup := seen[svc.Name]; dup {
			return fmt.Errorf("duplicate service name %q", svc.Name)
		}
		seen[svc.Name] = struct{}{}
		if _, err := parseDeps(svc.Dependencies); err != nil {
			return fmt.Errorf("services[%d] %q: %w", i, svc.Name, err)
		}
		if err := svc.Endpoints.validate(); err != nil {
			return fmt.Errorf("services[%d] %q: %w", i, svc.Name, err)
		}
	}
	return nil
}

func (e Endpoints) validate() error {
	seen := make(map[string]struct{}, len(e.GRPC))
	for i, ep := range e.GRPC {
		name := strings.TrimSpace(ep.Name)
		if !protoIdentRE.MatchString(name) {
			return fmt.Errorf("endpoints.grpc[%d].name %q must be a proto identifier", i, ep.Name)
		}
		key := pascalIdent(name)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate grpc endpoint %q", name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

type deps struct {
	postgres bool
	vault    bool
}

func parseDeps(list []string) (deps, error) {
	var d deps
	for _, raw := range list {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case depPostgresql, "postgres":
			d.postgres = true
		case depVault:
			d.vault = true
		case "":
			continue
		default:
			return deps{}, fmt.Errorf("unknown dependency %q (want postgresql or vault)", raw)
		}
	}
	return d, nil
}

// ProjectFromService fills template data from a catalog service.
func ProjectFromService(svc Service) (Project, error) {
	if !nameRE.MatchString(svc.Name) {
		return Project{}, fmt.Errorf("name %q must match %s", svc.Name, nameRE)
	}
	if strings.TrimSpace(svc.ID) == "" {
		return Project{}, fmt.Errorf("id must not be empty")
	}
	d, err := parseDeps(svc.Dependencies)
	if err != nil {
		return Project{}, err
	}
	return Project{
		Name:             svc.Name,
		ID:               strings.TrimSpace(svc.ID),
		Module:           "github.com/benthinnick/" + svc.Name,
		ServiceName:      svc.Name + "-api",
		PostgresUser:     svc.Name,
		PostgresDB:       svc.Name,
		PostgresPassword: svc.Name,
		HasPostgres:      d.postgres,
		HasVault:         d.vault,
	}, nil
}
