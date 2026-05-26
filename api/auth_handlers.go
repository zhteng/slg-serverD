package api

import (
	"net/http"
	"slg-serverD/auth"
	"slg-serverD/conf"

	"github.com/gin-gonic/gin"
)

// register 用户注册
func (s *Server) register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		ServerId int    `json:"server_id" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": "invalid request"})
		return
	}

	// 获取区服 ID：当前服务绑定的区服 ID（从运行配置取）
	serverId := conf.GetRuntimeConfig().ServerID

	user, err := s.userSvc.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "msg": err.Error()})
		return
	}

	// 生成双 token
	accessToken, _ := auth.GenerateAccessToken(user.Uid, serverId)
	refreshToken, _ := auth.GenerateRefreshToken(user.Uid, serverId)
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": gin.H{
			"user":          user,
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_in":    auth.AccessExpire.Seconds(),
		},
	})

}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": "invalid request"})
		return
	}

	// 1. 从数据库获取用户（需 UserService 提供 Authenticate 方法）
	user, err := s.userSvc.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": "invalid request"})
		return
	}

	serverId := conf.GetRuntimeConfig().ServerID

	// 生成双 token
	accessToken, err := auth.GenerateAccessToken(user.Uid, serverId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": "token generation failed"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.Uid, serverId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": "token generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": gin.H{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_in":    auth.AccessExpire.Seconds(),
		},
	})
}

// refreshToken 刷新 token，旧 refresh token 将被加入黑名单
func (s *Server) refreshToken(c *gin.Context) {
	// 中间件已校验 refresh token，并从上下文中获取 uid 和 jti
	uid := c.GetInt64("uid")
	oldJti := c.GetString("jti")

	// 将旧的 refresh token 加入黑名单（防止重用）
	ctx := c.Request.Context()
	_ = auth.AddToBlacklist(ctx, oldJti, auth.RefreshExpire)

	serverId := conf.GetRuntimeConfig().ServerID
	// 生成新的双 token
	accessToken, err := auth.GenerateAccessToken(uid, serverId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": "token generation failed"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(uid, serverId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": "token generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": gin.H{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_in":    auth.AccessExpire.Seconds(),
		},
	})
}

// logout 登出，将当前 access token 的 jti 加入黑名单
func (s *Server) logout(c *gin.Context) {
	jti := c.GetString("jti")
	err := auth.AddToBlacklist(c.Request.Context(), jti, auth.AccessExpire)
	if err != nil {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "logged out",
	})
}
