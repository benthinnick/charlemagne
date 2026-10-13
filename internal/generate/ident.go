package generate

import (
	"regexp"
	"strings"
	"unicode"
)

var protoIdentRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func pascalIdent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.ContainsAny(s, "-_") {
		parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
		var b strings.Builder
		for _, p := range parts {
			if p == "" {
				continue
			}
			b.WriteString(strings.ToUpper(p[:1]))
			b.WriteString(p[1:])
		}
		return b.String()
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func snakeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func exportedIdent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

func jsonName(s string) string {
	exp := exportedIdent(s)
	if exp == "" {
		return exp
	}
	return strings.ToLower(exp[:1]) + exp[1:]
}
