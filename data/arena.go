// -----------------------------------------------------------
// 模块:
// 功能:
// 作者: zteng
// 创建: 2026/5/26 20:27
// 文件: arena.go
// 版权: 仅限内部项目使用
// -----------------------------------------------------------
package data

// ArenaTarget 挑战目标面板信息
type ArenaTarget struct {
	Rank  int    `json:"rank"`
	Uid   int64  `json:"uid"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	Power int64  `json:"power"`
}

// ArenaPanel 竞技场面板数据
type ArenaPanel struct {
	MyRank  int            `json:"my_rank"`
	MyPower int64          `json:"my_power"`
	Merit   int            `json:"merit"`
	Targets []*ArenaTarget `json:"targets"` // 挑战目标
}

// ArenaChallengeResult 挑战结果
type ArenaChallengeResult struct {
	Victory bool  `json:"victory"`
	NewRank int   `json:"new_rank"`
	Merit   int   `json:"merit"`
	Reward  int64 `json:"reward"`
}

const (
	ArenaMeritReward     = 10  // 胜利奖励军功
	ArenaDailyChancesMax = 5   // 每日免费挑战次数
	ArenaBuyChancesMax   = 5   // 每日可购买次数上限
	ArenaBuyChancesCost  = 100 // 购买一次挑战次数花费金币

	ArenaRankKey          = "arena:rank"     // ZSET 排名 key (member=uid, score=rank)
	ArenaChancesKeyPrefix = "arena:chances:" // 挑战次数 key 前缀 + uid + :date
	ArenaBuyCntKeyPrefix  = "arena:buycnt:"  // 购买次数 key 前缀 + uid + :date
)
