package logmasterconfig

import _ "embed"

// These four files are the only maintained configuration sources packaged into the desktop EXE.

//go:embed projects.yaml
var ProjectsYAML []byte

//go:embed tasks.yaml
var TasksYAML []byte

//go:embed keywords.yaml
var KeywordsYAML []byte

//go:embed defaults.yaml
var DefaultsYAML []byte
