package api

/*
路由注册（引用 auth.AccessTokenAuth()）
*/
import (
	"net/http"
	"slg-serverD/auth"
	"slg-serverD/conf"
	"slg-serverD/game"
	"slg-serverD/ws"

	"github.com/gin-gonic/gin"
)

type Server struct {
	userSvc     *game.UserService
	allianceSvc *game.AllianceService
	buildingSvc *game.BuildingService
	troopsSvc   *game.TroopsService
	marchSvc    *game.MarchService
	hub         *ws.Hub
	serverSvc   *game.ServerService
	mapSvc      *game.MapService
	playerSvc   *game.PlayerService
	arenaSvc    *game.ArenaService

	internalSecret string
}

func NewServer(userSvc *game.UserService, allianceSvc *game.AllianceService, buildingSvc *game.BuildingService, troopsSvc *game.TroopsService, marchSvc *game.MarchService, hub *ws.Hub, serverSvc *game.ServerService, mapSvc *game.MapService, playerSvc *game.PlayerService, arenaSvc *game.ArenaService, internalSecret string) *Server {
	return &Server{userSvc: userSvc, allianceSvc: allianceSvc, buildingSvc: buildingSvc, troopsSvc: troopsSvc, marchSvc: marchSvc, hub: hub, serverSvc: serverSvc, mapSvc: mapSvc, playerSvc: playerSvc, arenaSvc: arenaSvc, internalSecret: internalSecret}
}

// SetupRouter 配置路由，返回 Gin 引擎
func (s *Server) SetupRouter() *gin.Engine {
	r := gin.New()                      // 默认不适用中间件
	r.Use(gin.Logger(), gin.Recovery()) // 全局中间件

	// 公开路由（登录获取token）
	r.POST("/register", s.register)
	r.POST("/login", s.login)
	r.POST("/refresh", s.refreshToken, auth.RefreshTokenAuth()) // 需要 refresh token

	// WebSocket 端点（单独认证）
	r.GET("/ws", s.websocketHandler)

	r.GET("/servers", s.listServers) // 客户端获取区服列表（公开）

	// 需要鉴权的路由组
	api := r.Group("/")
	api.Use(auth.AccessTokenAuth())
	{
		// 玩家信息
		api.GET("/user/info", s.userInfo)
		// 建筑相关
		api.POST("/building/upgrade", s.upgradeBuilding)

		// 部队训练
		api.POST("/troops/train", s.startTrain)
		// 行军
		api.POST("/march/launch", s.LaunchMarch)
		//api.POST("/march/cancel_gather", s.cancelGather)

		// 地图相关路由
		api.GET("/map/cell", s.GetCell)           // 查询地块
		api.POST("/map/relocate", s.RelocateCity) // 搬家

		// 军团
		//api.POST("/alliance/create", s.createAlliance)
		//api.POST("/alliance/kick", s.kickMember)

		api.POST("/logout", s.logout)

		api.GET("/admin/config", s.adminConfig) // 查看当前服配置（可增加简单鉴权）

		// 竞技场
		api.GET("/arena/panel", s.GetPanel)
		api.POST("/arena/challenge", s.Challenge)
		api.POST("/arena/buy_chance", s.BuyChance)
	}

	internal := r.Group("/internal")
	internal.Use(auth.InternalAuth(s.internalSecret))
	{
		internal.GET("/troops", s.internalGetTroops)
		internal.POST("/sync", s.internalSyncData)
	}

	return r
}

// login 处理登录，返回 JWT token（仅验证 uid 是否存在）
/*func (s *Server) login(c *gin.Context) {
	var req struct {
		Uid int64 `json:"uid" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "error": err.Error()})
		return
	}

	// 验证用户密码，这里简化：uid 必须 > 0
	if req.Uid <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "error": "invalid uid"})
		return
	}

	// 1. 查询数据库验证用户名和密码（或对接 OAuth/第三方登录）
	//user, err := s.userSvc.Authenticate(c.Request.Context(), req.Username, req.Password)
	//if err != nil {
	//	c.JSON(401, gin.H{"code": 0, "msg": "invalid credentials"})
	//	return
	//}

	// 2. 生成 token（使用真实的 user.uid）
	token, err := auth.GenerateToken(req.Uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"data": gin.H{
			"token": token,
		},
	})
}*/

// 获取玩家信息
func (s *Server) userInfo(c *gin.Context) {
	uid := c.GetInt64("uid")

	uInfo, err := s.playerSvc.GetUserInfo(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 0,
			"msg":  err.Error(),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": uInfo,
	})
}

// upgradeBuilding 建筑升级（需要 JWT 鉴权）
func (s *Server) upgradeBuilding(c *gin.Context) {
	// 从上下文获取 uid（由中间件注入）
	uidVal, exists := c.Get("uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "msg": "invalid uid"})
		return
	}

	uid, ok := uidVal.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "msg": "invalid uid"})
		return
	}

	var req struct {
		BuildingId string `json:"building_id" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": err.Error()})
		return
	}

	// 调用业务层
	err := s.buildingSvc.Upgrade(c.Request.Context(), uid, req.BuildingId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "building upgrade started",
	})
}

func (s *Server) startTrain(c *gin.Context) {
	uid := c.GetInt64("uid")
	var req struct {
		SoldierKey string `json:"soldier_key" binding:"required"`
		Num        int    `json:"num" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": err.Error()})
		return
	}

	// 调用业务层
	err := s.troopsSvc.UpdateTroops(c.Request.Context(), uid, req.SoldierKey, req.Num)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": "training started",
	})
}

// listServers 返回所有区服信息（从配置中读取)
func (s *Server) listServers(c *gin.Context) {
	server := s.serverSvc.GetAllServers()
	list := make([]gin.H, 0, len(server))
	for _, svr := range server {
		list = append(list, gin.H{
			"id":          svr.ID,
			"name":        svr.Name,
			"status":      svr.Status,
			"open_time":   svr.OpenTime,
			"max_players": svr.MaxPlayers,
			"http_port":   svr.HTTPPort,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": list,
	})
}

// adminConfig 返回当前服务运行时配置（脱敏）
func (s *Server) adminConfig(c *gin.Context) {
	rt := conf.GetRuntimeConfig()
	if rt == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "msg": "runtime config not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"msg":  "success",
		"data": rt.SafeInfo(),
	})
}
