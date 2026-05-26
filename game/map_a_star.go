package game

import "math"

/*
A* 寻路算法实现
*/

/*
// CalcMarchDuration 根据出发点和目标点坐标计算行军时间（秒）
// fromX, fromY: 出发地坐标
// toX, toY: 目的地坐标
// speedBonus: 科技加速比例，取值范围 [0, 1)。0 表示无加速，0.2 表示时间缩短 20%
// 返回值：向下取整的最终行军秒数

		local x = math.pow( targetCoord[1] - selfCoord[1] ,2)
	    local y = math.pow( targetCoord[2] - selfCoord[2] ,2 )
	    local baseSec = 120 -- 基础时间
	    local baseCellSec = 20  --单元格基础时间
	    local techRate = 0.05 * (1 + techBuff)
	    local sec =  ( math.sqrt(x +y ) * baseCellSec + baseSec )
*/
func CalcMarchDuration(fromX, fromY, toX, toY int, speedBonus float64) int {
	// 1. 计算坐标差
	dx := float64(fromX - toX)
	dy := float64(fromY - toY)

	// 1. 计算两点间直线距离（格子数），勾股定理
	distance := math.Sqrt(dx*dx + dy*dy)

	baseSec := 120.0    // 基础固定时间（出发准备时间）
	baseCellSec := 20.0 // 每格需要的时间

	// 1. 计算总秒数（速度加成提升）
	totalSec := (distance*baseCellSec + baseSec) / (1 + speedBonus)
	return int(math.Floor(totalSec))
}

// 科技减少行军时间
func CalcMarchDurationWithFixedReduce(fromX, fromY, toX, toY int, reduceSeconds int) int {
	tiles := math.Sqrt(float64((fromX-toX)*(fromX-toX) + (fromY-toY)*(fromY-toY)))
	base := tiles * 20
	if base < 120 {
		base = 120
	}

	base -= float64(reduceSeconds)
	if base < 0 {
		base = 0
	}
	return int(math.Floor(base))
}
