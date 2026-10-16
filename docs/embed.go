// Package docs 提供集中维护的 OpenAPI 文档和页面。
package docs

import _ "embed"

//go:embed openapi.yaml
var YAML []byte

//go:embed openapi.json
var JSON []byte

//go:embed index.html
var HTML []byte
