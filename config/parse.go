package config

import "gopkg.in/yaml.v2"

// Parse 将 config.yml 字节解析为配置；空/无效内容返回带默认值的安全配置而非 nil（C4）。
// 第二返回值为解析错误（若有）；无论解析成败，返回的配置均非 nil 且已填默认值，可安全解引用。
func Parse(yamlBytes []byte) (*Config, error) {
	cfg := &Config{}
	var err error
	if len(yamlBytes) > 0 {
		err = yaml.Unmarshal(yamlBytes, cfg)
	}
	if cfg.AutoUpdateModinfo.UpdateCheckInterval == 0 {
		cfg.AutoUpdateModinfo.UpdateCheckInterval = 10
	}
	if cfg.AutoUpdateModinfo.CheckInterval == 0 {
		cfg.AutoUpdateModinfo.CheckInterval = 5
	}
	return cfg, err
}
