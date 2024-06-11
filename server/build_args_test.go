package server

import (
	"testing"
)

func TestEncodeBuildArgs(t *testing.T) {
	external := NewStringSet()
	exports := NewStringSet()
	conditions := []string{"react-server"}
	external.Add("baz")
	external.Add("bar")
	exports.Add("baz")
	exports.Add("bar")
	buildArgsString := encodeBuildArgs(
		BuildArgs{
			alias: map[string]string{"a": "b"},
			deps: PkgSlice{
				Pkg{Name: "c", Version: "1.0.0"},
				Pkg{Name: "d", Version: "1.0.0"},
				Pkg{Name: "e", Version: "1.0.0"},
				Pkg{Name: "foo", Version: "1.0.0"}, // to be ignored
			},
			external:          external,
			exports:           exports,
			conditions:        conditions,
			jsxRuntime:        &Pkg{Version: "18.2.0", Name: "react"},
			externalRequire:   true,
			keepNames:         true,
			ignoreAnnotations: true,
		},
		Pkg{Name: "foo"},
		false,
	)
	args, err := decodeBuildArgs(nil, buildArgsString)
	if err != nil {
		t.Fatal(err)
	}
	if len(args.alias) != 1 || args.alias["a"] != "b" {
		t.Fatal("invalid alias")
	}
	if len(args.deps) != 3 {
		t.Fatal("invalid deps")
	}
	if args.external.Len() != 2 {
		t.Fatal("invalid external")
	}
	if args.exports.Len() != 2 {
		t.Fatal("invalid exports")
	}
	if len(args.conditions) != 1 || args.conditions[0] != "react-server" {
		t.Fatal("invalid conditions")
	}
	if args.jsxRuntime.String() != "react@18.2.0" {
		t.Fatal("invalid jsxRuntime")
	}
	if !args.externalRequire {
		t.Fatal("ignoreRequire should be true")
	}
	if !args.keepNames {
		t.Fatal("keepNames should be true")
	}
	if !args.ignoreAnnotations {
		t.Fatal("ignoreAnnotations should be true")
	}
}
