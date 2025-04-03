package dockerfiles

import _ "embed"

//go:embed typescript.tpl
var TSTemplate string

//go:embed python.tpl
var PythonTemplate string
