package data

type User struct {
	Uid              int64          `json:"uid"`
	Name             string         `json:"name"`
	Level            int            `json:"level"`
	Gold             int            `json:"gold"`
	Power            int64          `json:"power"`
	AllianceId       int64          `json:"alliance_id"`
	CityX            int            `json:"city_x"`
	CityY            int            `json:"city_y"`
	Resources        map[string]int `json:"resources"`
	PasswordHash     string         `json:"password_hash"`
	ServerId         int            `json:"server_id"`
	LastRelocateTime int64          `json:"last_relocate_time"` // 上次搬迁时间戳
}
