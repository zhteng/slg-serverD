package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

/*
地图相关 HTTP 接口
*/

// GetCell 查询单个地块
func (s *Server) GetCell(c *gin.Context) {
	x := c.GetInt("x")
	y := c.GetInt("y")
	cell, err := s.mapSvc.LoadOrCreateCell(c.Request.Context(), x, y)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  "error",
			"data": "cell not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": cell,
	})
}

func (s *Server) QueryMapCell(c *gin.Context) {
	xStr := c.DefaultQuery("x", "0")
	yStr := c.DefaultQuery("y", "0")
	x, _ := strconv.Atoi(xStr)
	y, _ := strconv.Atoi(yStr)
	cell, err := s.mapSvc.LoadOrCreateCell(c.Request.Context(), x, y)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  "error",
			"data": "cell not found",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": cell,
	})
}

// LaunchMarch 行军接口
func (s *Server) LaunchMarch(c *gin.Context) {
	var req struct {
		ToX    int            `json:"to_x"`
		Toy    int            `json:"to_y"`
		Type   int            `json:"type"`
		Troops map[string]int `json:"troops" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  err.Error(),
			"data": "invalid request",
		})
		return
	}

	uid := c.GetInt64("uid")
	march, err := s.marchSvc.LaunchMarch(c.Request.Context(), uid, req.ToX, req.Toy, req.Type, req.Troops, 0, 0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  "error",
			"data": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": march.ArriveTime,
	})
}

// POST /map/relocate  搬家
func (s *Server) RelocateCity(c *gin.Context) {
	var req struct {
		ToX int `json:"to_x" binding:"required"`
		ToY int `json:"to_y" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  err.Error(),
			"data": "invalid request",
		})
		return
	}
	uid := c.GetInt64("uid")
	if err := s.mapSvc.RelocateCity(c.Request.Context(), uid, req.ToY, req.ToY); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 0,
			"msg":  "error",
			"data": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
	})
}
