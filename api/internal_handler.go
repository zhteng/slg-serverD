package api

/*
原服内部接口实现（获取部队、同步数据）
*/
import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// 原服暴露内部 API（路由注册时使用 InternalAuth 中间件）
func (s *Server) internalGetTroops(c *gin.Context) {
	uidStr := c.Query("uid")
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}

	// 获取原有部队
	troops, err := s.troopsSvc.Load(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"data": troops,
	})
}

// 跨服服回写数据（更新部队、资源）
func (s *Server) internalSyncData(c *gin.Context) {
	var req struct {
		UID       int64          `json:"uid"`
		Troops    map[string]int `json:"troops"`
		Resources map[string]int `json:"resources"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}

	// 调用业务层更新
	err := s.userSvc.SyncCrossData(c.Request.Context(), req.UID, req.Troops, req.Resources)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
	})
}
