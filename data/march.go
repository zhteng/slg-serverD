package data

const (
	StatusGoing     = 1 // 行军中
	StatusArrived   = 2 // 已到达
	StatusReturning = 3 // 返回中
	StatusFinished  = 4 // 已结束
	StatusGathering = 5 // 采集中

	TypeAttack  = 1 // 攻击
	TypeGather  = 2 // 采集
	TypeStation = 3 // 驻扎
)

type MarchAttack struct {
	FromX      int            `json:"fx"`
	FromY      int            `json:"fy"`
	ToX        int            `json:"tx"`
	ToY        int            `json:"ty"`
	Type       int            `json:"type"`
	SkinId     int            `json:"skin"`
	Status     int            `json:"sts"`
	StartTime  int64          `json:"st"`
	ArriveTime int64          `json:"at"`
	RealArrive int64          `json:"rat"`
	Troops     map[string]int `json:"troops"` // 携带的士兵 {兵种:数量}

	// 采集专用
	GatherStartTime int64 `json:"gst,omitempty"`
	GatherDuration  int64 `json:"gdu,omitempty"`

	// 返程携带资源
	CarryResources int `json:"crs,omitempty"`
}

type SoldierQueue struct {
	Key      string `json:"key"`      // 兵种标识
	Total    int    `json:"total"`    // 训练数量
	FinishAt int64  `json:"finishAt"` // 完成时间戳
}

type Troops struct {
	Uid        int64                   `json:"uid"`
	Troops     map[string]int          `json:"troops"`      // 可用士兵
	Damaged    map[string]int          `json:"damaged"`     // 伤兵
	Attack     map[string]*MarchAttack `json:"attack"`      // 进行中的行军
	Queue      []*SoldierQueue         `json:"queue"`       // 训练队列
	UpdateTime int64                   `json:"update_time"` // 最近更新时间
	Version    int                     `json:"version"`     // 乐观锁版本号
}
