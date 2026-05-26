package conf

import (
	"os"

	"gopkg.in/yaml.v2"
)

type MapConfig struct {
	Width         int                   `yaml:"width"`
	Height        int                   `yaml:"height"`
	ObstacleRatio float64               `yaml:"obstacle_ratio"`
	Resources     []ResourceTemplate    `yaml:"resources"`
	Monsters      []MonstersTemplate    `yaml:"monsters"`
	StrongHolds   []StrongHoldsTemplate `yaml:"strong_holds"`
}

type ResourceTemplate struct {
	MinLevel    int     `yaml:"min_level"`
	MaxLevel    int     `yaml:"max_level"`
	Count       int     `yaml:"count"`
	MinDistance float64 `yaml:"min_distance"` // 最小间距
	Regenerate  int64   `yaml:"regenerate"`   // 枯竭后刷新时间（秒）
}

type MonstersTemplate struct {
	MinLevel    int     `yaml:"min_level"`
	MaxLevel    int     `yaml:"max_level"`
	Count       int     `yaml:"count"`
	MinDistance float64 `yaml:"min_distance"`
	Respawn     int64   `yaml:"respawn"` // 死亡后重生时间
}

type StrongHoldsTemplate struct {
	Level       int     `yaml:"level"`
	Count       int     `yaml:"count"`
	MinDistance float64 `yaml:"min_distance"`
}

func LoadMapConfig(path string) (*MapConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m MapConfig
	err = yaml.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
