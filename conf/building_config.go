package conf

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// 单个建筑的元数据
type BuildingMeta struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	MaxLevel int    `yaml:"max_level"`
	BaseTime int64  `yaml:"base_time"`
}

// 全部建筑配置集合
type BuildingConfig struct {
	Buildings map[string]*BuildingMeta `yaml:"buildings"`
}

func LoadBuildingConfig(path string) (*BuildingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	var cfg BuildingConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
