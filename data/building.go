package data

// Building 玩家建筑数据（内存/缓存模型）
type Building struct {
	Uid       int64            `json:"uid"`
	Info      map[string]int   `json:"info"`       // 建筑ID → 等级
	Queue     map[string]int64 `json:"queue"`      // 建造队列：建筑ID → 完成时间戳(Unix)
	UpdatedAt int64            `json:"updated_at"` // 最后更新时间戳
}
