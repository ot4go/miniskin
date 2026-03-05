package miniskin

import _ "embed"

//go:embed templates/bucket_default.go.tmpl
var defaultBucketTmpl string

//go:embed templates/embed_default.go.tmpl
var defaultEmbedTmpl string
