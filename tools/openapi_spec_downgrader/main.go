package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	in := "/workspaces/amalthea/internal/remote/firecrest/openapi_spec_original.yaml"
	out := "/workspaces/amalthea/internal/remote/firecrest/openapi_spec_test.yaml"
	contents, err := os.ReadFile(in)
	if err != nil {
		log.Fatalln(err)
	}
	doc := map[string]any{}
	err = yaml.Unmarshal(contents, doc)
	if err != nil {
		log.Fatalln(err)
	}

	transformed, err := transformNode(doc)
	if err != nil {
		log.Fatalln(err)
	}
	newContents, err := yaml.Marshal(transformed)
	if err != nil {
		log.Fatalln(err)
	}

	err = os.Remove(out)
	if err != nil {
		log.Fatalln(err)
	}
	err = os.WriteFile(out, newContents, 0644)
	if err != nil {
		log.Fatalln(err)
	}
}

func transformNode(in any) (out any, err error) {
	asMap, ok := in.(map[string]any)
	if ok {
		return transformMap(asMap)
	}

	asArr, ok := in.([]any)
	if ok {
		return transformArray(asArr)
	}

	out = in
	return out, nil
}

func transformMap(in map[string]any) (out map[string]any, err error) {
	out = map[string]any{}
	for key := range in {
		t, err := transformNode(in[key])
		if err != nil {
			return out, err
		}
		if key == "anyOf" {
			canSimplify, tt, err := simplifyAnyOf(t)
			if err != nil {
				return out, err
			}
			if canSimplify {
				for subKey := range tt {
					out[subKey] = tt[subKey]
				}
			} else {
				out[key] = t
			}
		} else {
			out[key] = t
		}
	}
	return out, nil
}

func transformArray(in []any) (out []any, err error) {
	out = []any{}
	hasTypeNull := false
	for idx := range in {
		if isTypeNull(in[idx]) {
			hasTypeNull = true
		} else {
			t, err := transformNode(in[idx])
			if err != nil {
				return out, err
			}
			out = append(out, t)
		}
	}
	// log.Printf("hasTypeNull: %t\n", hasTypeNull)
	if hasTypeNull {
		for idx := range out {
			t, err := makeNullable(out[idx])
			if err != nil {
				return out, err
			}
			out[idx] = t
		}
	}
	return out, nil
}

func isTypeNull(node any) bool {
	asMap, ok := node.(map[string]any)
	if !ok {
		return false
	}
	return asMap["type"] == "null"
}

func makeNullable(in any) (out map[string]any, err error) {
	asMap, ok := in.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("cannot make non-map node nullable")
	}
	out = asMap
	out["nullable"] = true
	return out, nil
}

func simplifyAnyOf(in any) (canSimplify bool, out map[string]any, err error) {
	asArr, ok := in.([]any)
	if !ok {
		return false, nil, fmt.Errorf("anyOf node should contain a list")
	}
	if len(asArr) == 1 {
		asMap, ok := asArr[0].(map[string]any)
		if ok {
			return true, asMap, nil
		}
	}
	return false, nil, nil
}
