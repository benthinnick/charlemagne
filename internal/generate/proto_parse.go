package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emicklei/proto"
)

type protoFile struct {
	GoImport string
	GoPkg    string
	Services []protoService
	Messages map[string]protoMessage
}

type protoService struct {
	Name string
	RPCs []protoRPC
}

type protoRPC struct {
	Name     string
	Request  string
	Response string
}

type protoMessage struct {
	Name   string
	Fields []protoField
}

type protoField struct {
	Name     string
	GoName   string
	JSONName string
	GoType   string
	Repeated bool
	Message  bool
	Map      bool
	KeyType  string
}

var protoScalars = map[string]string{
	"double":   "float64",
	"float":    "float32",
	"int32":    "int32",
	"int64":    "int64",
	"uint32":   "uint32",
	"uint64":   "uint64",
	"sint32":   "int32",
	"sint64":   "int64",
	"fixed32":  "uint32",
	"fixed64":  "uint64",
	"sfixed32": "int32",
	"sfixed64": "int64",
	"bool":     "bool",
	"string":   "string",
	"bytes":    "[]byte",
}

func parseServiceProtos(dir, fallbackImport, fallbackPkg string) (protoFile, error) {
	out := protoFile{
		GoImport: fallbackImport,
		GoPkg:    fallbackPkg,
		Messages: map[string]protoMessage{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return protoFile{}, err
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".proto") {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return protoFile{}, err
		}
		def, err := proto.NewParser(strings.NewReader(string(body))).Parse()
		if err != nil {
			return protoFile{}, fmt.Errorf("parse %s: %w", path, err)
		}
		if err := absorbProto(&out, def); err != nil {
			return protoFile{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	return out, nil
}

func absorbProto(out *protoFile, def *proto.Proto) error {
	var streamErr error
	proto.Walk(def,
		proto.WithOption(func(o *proto.Option) {
			if o.Name == "go_package" && o.Constant.Source != "" {
				imp, pkg := splitGoPackage(o.Constant.Source)
				if imp != "" {
					out.GoImport = imp
				}
				if pkg != "" {
					out.GoPkg = pkg
				}
			}
		}),
		proto.WithService(func(s *proto.Service) {
			ps := protoService{Name: s.Name}
			for _, el := range s.Elements {
				rpc, ok := el.(*proto.RPC)
				if !ok {
					continue
				}
				if rpc.StreamsRequest || rpc.StreamsReturns {
					streamErr = fmt.Errorf("streaming rpc %s is not supported", rpc.Name)
					return
				}
				ps.RPCs = append(ps.RPCs, protoRPC{
					Name:     rpc.Name,
					Request:  unqualified(rpc.RequestType),
					Response: unqualified(rpc.ReturnsType),
				})
			}
			out.Services = append(out.Services, ps)
		}),
		proto.WithMessage(func(m *proto.Message) {
			out.Messages[m.Name] = parseMessage(m)
		}),
	)
	if streamErr != nil {
		return streamErr
	}
	for _, svc := range out.Services {
		for _, rpc := range svc.RPCs {
			if rpc.Request == "" || rpc.Response == "" {
				return fmt.Errorf("rpc %s must have request and response types", rpc.Name)
			}
		}
	}
	return nil
}

func parseMessage(m *proto.Message) protoMessage {
	msg := protoMessage{Name: m.Name}
	for _, el := range m.Elements {
		switch f := el.(type) {
		case *proto.NormalField:
			goType, isMsg := goFieldType(f.Type, f.Repeated, false, "")
			msg.Fields = append(msg.Fields, protoField{
				Name:     f.Name,
				GoName:   exportedIdent(f.Name),
				JSONName: jsonName(f.Name),
				GoType:   goType,
				Repeated: f.Repeated,
				Message:  isMsg,
			})
		case *proto.MapField:
			goType, isMsg := goFieldType(f.Type, false, true, f.KeyType)
			msg.Fields = append(msg.Fields, protoField{
				Name:     f.Name,
				GoName:   exportedIdent(f.Name),
				JSONName: jsonName(f.Name),
				GoType:   goType,
				Map:      true,
				Message:  isMsg,
				KeyType:  mapKeyGo(f.KeyType),
			})
		}
	}
	return msg
}

func goFieldType(protoType string, repeated, isMap bool, keyType string) (string, bool) {
	t := unqualified(protoType)
	goType, scalar := protoScalars[t]
	isMsg := !scalar
	if !scalar {
		goType = exportedIdent(t)
	}
	if isMap {
		return "map[" + mapKeyGo(keyType) + "]" + goType, isMsg
	}
	if repeated {
		return "[]" + goType, isMsg
	}
	return goType, isMsg
}

func mapKeyGo(t string) string {
	if g, ok := protoScalars[t]; ok {
		return g
	}
	return "string"
}

func unqualified(t string) string {
	t = strings.TrimSpace(t)
	if i := strings.LastIndex(t, "."); i >= 0 {
		return t[i+1:]
	}
	return t
}

func splitGoPackage(s string) (imp, pkg string) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, ";"); i >= 0 {
		return s[:i], s[i+1:]
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s, s[i+1:]
	}
	return s, s
}

func (p protoFile) allRPCs() []protoRPC {
	var out []protoRPC
	for _, svc := range p.Services {
		out = append(out, svc.RPCs...)
	}
	return out
}
