package generate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runBufGenerate(protoRoot, dest, serviceName string) error {
	if _, err := exec.LookPath("buf"); err != nil {
		return fmt.Errorf("buf is required for build (install: go install github.com/bufbuild/buf/cmd/buf@latest): %w", err)
	}
	for _, plugin := range []string{"protoc-gen-go", "protoc-gen-go-grpc"} {
		if _, err := exec.LookPath(plugin); err != nil {
			return fmt.Errorf("%s is required for build (install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest): %w", plugin, err)
		}
	}

	if _, err := os.Stat(filepath.Join(protoRoot, "buf.yaml")); os.IsNotExist(err) {
		body := "version: v2\nmodules:\n  - path: .\n"
		if err := os.WriteFile(filepath.Join(protoRoot, "buf.yaml"), []byte(body), 0o644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	outDir, err := filepath.Abs(filepath.Join(dest, "internal", "proto"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	tmpl := filepath.Join(os.TempDir(), "charlemagne-buf.gen.yaml")
	yaml := fmt.Sprintf("version: v2\nplugins:\n  - local: protoc-gen-go\n    out: %s\n    opt: paths=source_relative\n  - local: protoc-gen-go-grpc\n    out: %s\n    opt:\n      - paths=source_relative\n      - require_unimplemented_servers=true\n",
		filepath.ToSlash(outDir), filepath.ToSlash(outDir))
	if err := os.WriteFile(tmpl, []byte(yaml), 0o644); err != nil {
		return err
	}

	cmd := exec.Command("buf", "generate", "--template", tmpl, "--path", filepath.ToSlash(filepath.Join("services", serviceName)))
	cmd.Dir = protoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("buf generate: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
