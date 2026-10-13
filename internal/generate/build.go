package generate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// BuildOptions selects which stages to run.
type BuildOptions struct {
	Stage   string
	From    string
	Stages  []string
	SkipBuf bool
	Log     io.Writer
}

// Build runs named stages against an existing service.
func Build(dest string, svc Service, catalogPath string, opts BuildOptions) error {
	proj, err := ProjectFromService(svc)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dest, "go.mod")); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("destination %s is not a generated service (missing go.mod); run charlemagne new %s first", dest, svc.Name)
		}
		return err
	}

	stages, err := selectStages(opts)
	if err != nil {
		return err
	}

	for _, stage := range stages {
		if err := runStage(dest, svc, proj, catalogPath, stage, opts); err != nil {
			return err
		}
	}
	return nil
}

func runStage(dest string, svc Service, proj Project, catalogPath, stage string, opts BuildOptions) error {
	switch stage {
	case StageProto:
		logStage(opts, "proto")
		_, err := mergeProtoStubs(catalogPath, svc, proj)
		return err
	case StageProtobuf:
		logStage(opts, "protobuf")
		return runBufGenerate(protoRoot(catalogPath), dest, svc.Name)
	case StageRPC:
		logStage(opts, "rpc")
		return writeRPCFromProto(dest, svc, proj, catalogPath)
	case StageORM:
		return runORM(dest, proj, opts)
	case StageTidy:
		logStage(opts, "tidy")
		return runGo(dest, "mod", "tidy")
	default:
		return fmt.Errorf("unknown stage %q", stage)
	}
}

func writeRPCFromProto(dest string, svc Service, proj Project, catalogPath string) error {
	fallbackImport := proj.Module + "/internal/proto/services/" + svc.Name
	fallbackPkg := strings.ReplaceAll(svc.Name, "-", "") + "v1"
	pf, err := parseServiceProtos(serviceProtoDir(catalogPath, svc.Name), fallbackImport, fallbackPkg)
	if err != nil {
		return err
	}
	if pf.GoImport == "" {
		pf.GoImport = fallbackImport
	}
	return writeRPCLayers(dest, proj, pf)
}

func logStage(opts BuildOptions, msg string) {
	w := opts.Log
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "==> %s\n", msg)
}
