package conf

import (
	"os"

	"gopkg.in/yaml.v2"
)

type SoldiersMeta struct {
	ID        string `yaml:"id"`
	Name      string `yaml:"name"`
	TrainTime int64  `yaml:"train_time"`
}

type TroopsConfig struct {
	Soldiers map[string]SoldiersMeta `yaml:"soldiers"`
}

func LoadTroopsConfig(path string) (*TroopsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg TroopsConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
