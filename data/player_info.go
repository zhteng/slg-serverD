// -----------------------------------------------------------
// 模块:
// 功能:
// 作者: zteng
// 创建: 2026/5/26 18:15
// 文件: player_info.go
// 版权: 仅限内部项目使用
// -----------------------------------------------------------
package data

type PlayerInfo struct {
	User   *User   `json:"user"`
	Troops *Troops `json:"troops"`
}
