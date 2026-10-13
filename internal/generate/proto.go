package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emicklei/proto"
)

func protoRoot(catalogPath string) string {
	return filepath.Join(filepath.Dir(catalogPath), "proto")
}

func serviceProtoDir(catalogPath, name string) string {
	return filepath.Join(protoRoot(catalogPath), "services", name)
}

func serviceProtoFile(catalogPath, name string) string {
	return filepath.Join(serviceProtoDir(catalogPath, name), name+".proto")
}

func goPackageOption(module, name string) string {
	return module + "/internal/proto/services/" + name + ";" + strings.ReplaceAll(name, "-", "") + "v1"
}

func mergeProtoStubs(catalogPath string, svc Service, proj Project) (string, error) {
	dir := serviceProtoDir(catalogPath, svc.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := serviceProtoFile(catalogPath, svc.Name)
	serviceName := pascalIdent(svc.Name)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(newProtoFile(svc, proj, serviceName)), 0o644); err != nil {
			return "", err
		}
		return path, nil
	} else if err != nil {
		return "", err
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	merged, err := mergeProtoContent(string(body), serviceName, svc, proj)
	if err != nil {
		return "", fmt.Errorf("merge proto %s: %w", path, err)
	}
	if merged != string(body) {
		if err := os.WriteFile(path, []byte(merged), 0o644); err != nil {
			return "", err
		}
	}
	return path, nil
}

func newProtoFile(svc Service, proj Project, serviceName string) string {
	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\n\n")
	b.WriteString("package " + svc.Name + ".v1;\n\n")
	b.WriteString("option go_package = \"" + goPackageOption(proj.Module, svc.Name) + "\";\n\n")
	b.WriteString("service " + serviceName + " {\n")
	for _, ep := range svc.Endpoints.GRPC {
		rpc := pascalIdent(ep.Name)
		b.WriteString("  rpc " + rpc + "(" + rpc + "Request) returns (" + rpc + "Response);\n")
	}
	b.WriteString("}\n")
	for _, ep := range svc.Endpoints.GRPC {
		rpc := pascalIdent(ep.Name)
		b.WriteString("\nmessage " + rpc + "Request {}\n")
		b.WriteString("message " + rpc + "Response {}\n")
	}
	return b.String()
}

func mergeProtoContent(src, serviceName string, svc Service, proj Project) (string, error) {
	def, err := proto.NewParser(strings.NewReader(src)).Parse()
	if err != nil {
		return "", err
	}

	existingRPC := map[string]struct{}{}
	existingMsg := map[string]struct{}{}
	hasService := false
	proto.Walk(def,
		proto.WithService(func(s *proto.Service) {
			if s.Name == serviceName {
				hasService = true
			}
		}),
		proto.WithRPC(func(r *proto.RPC) {
			existingRPC[r.Name] = struct{}{}
		}),
		proto.WithMessage(func(m *proto.Message) {
			existingMsg[m.Name] = struct{}{}
		}),
	)

	out := src
	if !strings.Contains(out, "option go_package") {
		opt := "option go_package = \"" + goPackageOption(proj.Module, svc.Name) + "\";\n"
		if i := strings.Index(out, "package "); i >= 0 {
			if nl := strings.Index(out[i:], "\n"); nl >= 0 {
				insert := i + nl + 1
				out = out[:insert] + "\n" + opt + out[insert:]
			}
		} else {
			out = opt + out
		}
	}

	var missingRPC []string
	var missingMsg []string
	for _, ep := range svc.Endpoints.GRPC {
		rpc := pascalIdent(ep.Name)
		if _, ok := existingRPC[rpc]; !ok {
			missingRPC = append(missingRPC, rpc)
			existingRPC[rpc] = struct{}{}
		}
		for _, msg := range []string{rpc + "Request", rpc + "Response"} {
			if _, ok := existingMsg[msg]; !ok {
				missingMsg = append(missingMsg, msg)
				existingMsg[msg] = struct{}{}
			}
		}
	}

	if !hasService {
		var block strings.Builder
		block.WriteString("\nservice " + serviceName + " {\n")
		for _, rpc := range missingRPC {
			block.WriteString("  rpc " + rpc + "(" + rpc + "Request) returns (" + rpc + "Response);\n")
		}
		block.WriteString("}\n")
		out = strings.TrimRight(out, "\n") + "\n" + block.String()
		missingRPC = nil
	}

	for _, rpc := range missingRPC {
		line := "  rpc " + rpc + "(" + rpc + "Request) returns (" + rpc + "Response);\n"
		inserted, err := insertRPC(out, serviceName, line)
		if err != nil {
			return "", err
		}
		out = inserted
	}

	if len(missingMsg) > 0 {
		var block strings.Builder
		for _, msg := range missingMsg {
			block.WriteString("\nmessage " + msg + " {}\n")
		}
		out = strings.TrimRight(out, "\n") + "\n" + block.String()
	}
	return out, nil
}

func insertRPC(src, serviceName, rpcLine string) (string, error) {
	needle := "service " + serviceName
	idx := strings.Index(src, needle)
	if idx < 0 {
		return "", fmt.Errorf("service %s not found", serviceName)
	}
	brace := strings.Index(src[idx:], "{")
	if brace < 0 {
		return "", fmt.Errorf("service %s has no opening brace", serviceName)
	}
	start := idx + brace
	depth := 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[:i] + rpcLine + src[i:], nil
			}
		}
	}
	return "", fmt.Errorf("service %s has no closing brace", serviceName)
}
