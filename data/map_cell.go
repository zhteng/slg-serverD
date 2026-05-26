package data

import (
	"sync"
	"time"
)

const (
	CellTypeEmpty      = 0 // 空地
	CellTypeResource   = 1 // 资源
	CellTypeMonster    = 2 // 野怪
	CellTypeStrongHold = 3 // 据点
	CellTypeObstacle   = 4 // 障碍（山脉、河流）
	CellTypeUser       = 5 // 玩家
)

// MapCell 单个地块
type MapCell struct {
	ID       int64                  `json:"id"` // x<<32 | y
	X        int                    `json:"x"`
	Y        int                    `json:"y"`
	MType    int                    `json:"mtype"`
	Level    int                    `json:"level"`
	Data     map[string]interface{} `json:"data"` // 扩展属性（储量、刷新时间等）
	Owner    int64                  `json:"oid"`  // 0=无主
	Name     string                 `json:"name"`
	Power    int64                  `json:"power"`
	Rank     int                    `json:"rank"`
	Alliance int64                  `json:"alliance"`
	Protect  int64                  `json:"protect"`
	Pic      string                 `json:"pic"`

	// 运行时状态（不持久化）
	Mu      sync.RWMutex
	Queue   []*MarchQueueItem // 行军队列（按到达时间排序）
	IsDirty bool              // 标记是否需要同步到 DB
}

// MarchQueueItem 等待队列项
type MarchQueueItem struct {
	MarchKey  string // 行军唯一标识
	Uid       int64
	Troops    map[string]int
	ArriveAt  time.Time // 到达时间
	Type      int       // 1攻击 2采集 3驻扎
	ActionEnd time.Time // 采集完成时间
}

// 分片：40x40 格子
const ShardWidth = 40
const ShardHeight = 40
const MapWidth = 600
const MapHeight = 600
const ShardCountX = MapWidth / ShardWidth   // 15
const ShardCountY = MapHeight / ShardHeight // 15

// MapShard 分片锁
type MapShard struct {
	ShardX int
	ShardY int
	Cells  map[int64]*MapCell
	Mu     sync.RWMutex
}

// GenCellID 根据坐标生成唯一 ID
func GenCellID(x, y int) int64 {
	return int64(x)<<32 | int64(y)
}

func ParseCellID(id int64) (int, int) {
	x := int(id >> 32)        // 右移 32 位提取高位的 x
	y := int(id & 0xFFFFFFFF) // 通过掩码提取低 32 位的 y
	return x, y
}

// IsOccupiable 判断地块是否可被玩家占领（用于搬家目标）
func (cell *MapCell) IsOccupiable() bool {
	return cell.Owner == 0 && (cell.MType == CellTypeEmpty)
}
