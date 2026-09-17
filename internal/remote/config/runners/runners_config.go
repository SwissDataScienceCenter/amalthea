package runners

type RunnersConfig struct{}

func GetConfig() (cfg RunnersConfig) {
	cfg = RunnersConfig{}
	return cfg
}

func (cfg *RunnersConfig) Validate() error {
	return nil
}
