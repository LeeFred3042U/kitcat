package remote

import (
	"fmt"
	"strings"
)

// Refspec represents a fetch/push refspec.
// This implementation is intentionally minimal: it supports an optional leading
// '+' (force) and a single '*' wildcard in both Src and Dst.
type Refspec struct {
	Force bool
	Src   string
	Dst   string
}

type RefMapping struct {
	Src string
	Dst string
}

func ParseRefspec(s string) (Refspec, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Refspec{}, fmt.Errorf("empty refspec")
	}

	spec := Refspec{}
	if strings.HasPrefix(raw, "+") {
		spec.Force = true
		raw = strings.TrimPrefix(raw, "+")
	}

	src, dst, hasColon := strings.Cut(raw, ":")
	src = strings.TrimSpace(src)
	dst = strings.TrimSpace(dst)

	if src == "" {
		return Refspec{}, fmt.Errorf("invalid refspec %q", s)
	}
	if hasColon && dst == "" {
		return Refspec{}, fmt.Errorf("invalid refspec %q", s)
	}
	spec.Src = src
	spec.Dst = dst
	return spec, nil
}

// ExpandRefspec expands a possibly-wildcard refspec against advertised remote refs.
// Only remote refs are matched (Src side) because fetch maps remote -> local.
func ExpandRefspec(spec Refspec, remoteRefs []RefInfo) ([]RefMapping, error) {
	// Non-wildcard: map Src to Dst directly.
	if !strings.Contains(spec.Src, "*") {
		if spec.Dst == "" {
			return nil, fmt.Errorf("refspec %q missing destination", spec.Src)
		}
		return []RefMapping{{Src: spec.Src, Dst: spec.Dst}}, nil
	}

	// Minimal wildcard support: exactly one '*' in src and dst.
	if strings.Count(spec.Src, "*") != 1 || strings.Count(spec.Dst, "*") != 1 {
		return nil, fmt.Errorf("refspec wildcard must contain exactly one '*' in src and dst")
	}

	srcPrefix, srcSuffix, _ := strings.Cut(spec.Src, "*")
	dstPrefix, dstSuffix, _ := strings.Cut(spec.Dst, "*")

	var out []RefMapping
	for _, r := range remoteRefs {
		if !strings.HasPrefix(r.Name, srcPrefix) || !strings.HasSuffix(r.Name, srcSuffix) {
			continue
		}
		mid := strings.TrimSuffix(strings.TrimPrefix(r.Name, srcPrefix), srcSuffix)
		out = append(out, RefMapping{
			Src: r.Name,
			Dst: dstPrefix + mid + dstSuffix,
		})
	}
	return out, nil
}

