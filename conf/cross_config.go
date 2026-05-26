package conf

import (
	"os"

	"gopkg.in/yaml.v2"
)

type CrossConfig struct {
	Port           int            `yaml:"port"`
	ServerMap      map[int]string `yaml:"servers"`
	InternalSecret string         `yaml:"internal_secret"`
}

func LoadCrossConfig(path string) (*CrossConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg CrossConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	// 读取文件，如果环境变量有则覆盖 Secret
	if env := os.Getenv("INTERNAL_SECRET"); env != "" {
		cfg.InternalSecret = env
	}

	cfg.InternalSecret = "aaa"
	return &cfg, nil
}
