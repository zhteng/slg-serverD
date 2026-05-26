// -----------------------------------------------------------
// 模块:
// 功能:
// 作者: zteng
// 创建: 2026/5/26 23:31
// 文件: arena_handler.go
// 版权: 仅限内部项目使用
// -----------------------------------------------------------
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetPanel 获取竞技场面板
func (s *Server) GetPanel(c *gin.Context) {
	uid := c.GetInt64("uid")
	panel, err := s.arenaSvc.GetPanel(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": panel,
	})
}

// Challenge 挑战玩家
func (s *Server) Challenge(c *gin.Context) {
	uid := c.GetInt64("uid")
	var req struct {
		TargetUid int64 `json:"target_uid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}
	result, err := s.arenaSvc.Challenge(c.Request.Context(), uid, req.TargetUid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": result,
	})
}

// BuyChance 购买挑战次数
func (s *Server) BuyChance(c *gin.Context) {
	uid := c.GetInt64("uid")
	if err := s.arenaSvc.BuyChance(c.Request.Context(), uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": "ok",
	})
}
