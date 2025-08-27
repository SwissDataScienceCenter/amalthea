//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -generate types,client,spec -package firecrest -o firecrest_gen.go openapi_spec_downgraded.yaml

package firecrest
