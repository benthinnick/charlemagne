package generate

import (
	"fmt"
	"slices"
	"strings"
)

const (
	StageProto    = "proto"
	StageProtobuf = "protobuf"
	StageRPC      = "rpc"
	StageORM      = "orm"
	StageTidy     = "tidy"
)

var allStages = []string{StageProto, StageProtobuf, StageRPC, StageORM, StageTidy}

func selectStages(opts BuildOptions) ([]string, error) {
	if opts.Stage != "" && opts.From != "" {
		return nil, fmt.Errorf("use --stage or --from, not both")
	}
	if err := validateStage(opts.Stage); err != nil {
		return nil, err
	}
	if err := validateStage(opts.From); err != nil {
		return nil, err
	}

	var list []string
	switch {
	case len(opts.Stages) > 0:
		for _, s := range opts.Stages {
			if err := validateStage(s); err != nil {
				return nil, err
			}
			if s == "" {
				continue
			}
			list = append(list, s)
		}
	case opts.Stage != "":
		list = []string{opts.Stage}
	case opts.From != "":
		i := slices.Index(allStages, opts.From)
		list = append([]string{}, allStages[i:]...)
	default:
		list = append([]string{}, allStages...)
	}

	if opts.SkipBuf {
		filtered := list[:0]
		for _, s := range list {
			if s == StageProtobuf {
				continue
			}
			filtered = append(filtered, s)
		}
		list = filtered
	}
	return list, nil
}

func validateStage(name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	if slices.Contains(allStages, name) {
		return nil
	}
	return fmt.Errorf("unknown stage %q (want %s)", name, strings.Join(allStages, ", "))
}
