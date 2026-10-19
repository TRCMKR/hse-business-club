package hbcv1

import _ "embed" // embed is required for go:embed

//go:embed hbc.swagger.json
var swaggerJSON []byte

// SwaggerJSON returns the OpenAPI v2 document generated from hbc.proto.
func SwaggerJSON() []byte {
	return swaggerJSON
}
