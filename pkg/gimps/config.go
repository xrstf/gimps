package gimps

type Config struct {
	ProjectName string      `yaml:"projectName"`
	ImportOrder []string    `yaml:"importOrder"`
	Sets        []Set       `yaml:"sets"`
	AliasRules  []AliasRule `yaml:"aliasRules"`
}

func setDefaults(c *Config) {
	if len(c.ImportOrder) == 0 {
		c.ImportOrder = []string{SetStd, SetProject, SetExternal}
	}
}
