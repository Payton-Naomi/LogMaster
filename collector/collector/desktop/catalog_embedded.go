package main

import logmasterconfig "logmaster-agent/agent/configs/logmaster"

func loadEmbeddedCatalog() (CatalogConfig, error) {
	return parseMaintainedCatalog(logmasterconfig.ProjectsYAML, logmasterconfig.TasksYAML, logmasterconfig.KeywordsYAML)
}
