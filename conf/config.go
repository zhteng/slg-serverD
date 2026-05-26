package conf

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server         ServerConfig `yaml:"server"`
	DB             DBConfig     `yaml:"db"`
	Redis          RedisConfig  `yaml:"redis"`
	InternalSecret string       `yaml:"internal_secret"`
	Servers        []ServerInfo `yaml:"servers"`
}

type ServerConfig struct {
	HTTPPort int `yaml:"http_port"`
}

type DBConfig struct {
	DSN string `yaml:"dsn"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
}

type ServerInfo struct {
	ID         int      `yaml:"id"`
	Name       string   `yaml:"name"`
	Status     int      `yaml:"status"` // 1=正常 0=维护
	OpenTime   int64    `yaml:"open_time"`
	MaxPlayers int      `yaml:"max_players"`
	HTTPPort   int      `yaml:"http_port"` // 区服对外端口（可选）
	DB         DBConfig `yaml:"db"`
	Redis      struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		Prefix   string `yaml:"prefix"`
	} `yaml:"redis"`
	Cross   string `yaml:"cross"`
	ChatURL string `yaml:"chatUrl"`
	Kafka   string `yaml:"kafka"`
}

func Load(path string) (*Config, error) {
	dataR, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(dataR, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
