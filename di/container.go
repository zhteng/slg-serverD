package di

import (
	"database/sql"
	"slg-serverD/auth"
	"slg-serverD/cache"
	"slg-serverD/conf"
	"slg-serverD/db"
	"slg-serverD/game"
	"slg-serverD/ws"

	"github.com/go-redis/redis/v8"
)

// Container 集中存放所有服务实例
type Container struct {
	UserService     *game.UserService
	AllianceService *game.AllianceService
	BuildingService *game.BuildingService
	TroopsService   *game.TroopsService
	MarchService    *game.MarchService
	Hub             *ws.Hub
	ServerService   *game.ServerService
	MapService      *game.MapService
	PlayerService   *game.PlayerService
	ArenaService    *game.ArenaService
}

// NewContainer 根据数据库和 Redis 客户端构建所有服务
func NewContainer(dbConn *sql.DB, rdb *redis.Client, cfg *conf.Config, rt *conf.RuntimeConfig, buildingConf *conf.BuildingConfig, troopsConf *conf.TroopsConfig) *Container {
	auth.InitRedis(rdb)
	// 锁服务
	lockSvc := game.NewLockService(rdb)
	//pushSvc := game.NewDefaultPushService()
	hub := ws.NewHub()
	pushSvc := ws.NewWSPushService(hub)

	// 区服
	serverSvc := game.NewServerService(cfg, rt)

	// 基础 repo 和 cache
	userRepo := db.NewUserRepo(dbConn)
	userCache := cache.NewUserCache(rdb)
	userSvc := game.NewUserService(userRepo, userCache, lockSvc)

	// 军团服务需要 UserService 和 LockService
	allianceRepo := db.NewAllianceRepo(dbConn)
	allianceCache := cache.NewAllianceCache(rdb)
	allianceSvc := game.NewAllianceService(allianceRepo, allianceCache, userSvc, lockSvc)

	// 建筑
	buildingRepo := db.NewBuildingRepo(dbConn)
	buildingCache := cache.NewBuildingCache(rdb)
	buildingSvc := game.NewBuildingService(buildingRepo, buildingCache, lockSvc, buildingConf, pushSvc)

	// 部队
	troopsRepo := db.NewTroopsRepo(dbConn)
	troopsCache := cache.NewTroopsCache(rdb)
	troopsSvc := game.NewTroopsService(troopsRepo, troopsCache, lockSvc, rdb, pushSvc, troopsConf)

	mapCfg, _ := conf.LoadMapConfig("config/map_config.yaml")
	mapCache := cache.NewMapCache(rdb)
	mapRepo := db.NewMapRepo(dbConn)
	mapSvc := game.NewMapService(mapCfg, mapCache, mapRepo, rdb, lockSvc, userSvc)

	marchSvc := game.NewMarchService(troopsSvc, userSvc, mapSvc, lockSvc, rdb, pushSvc)

	// 玩家所有信息
	playerSvc := game.NewPlayerService(userSvc, troopsSvc, buildingSvc)

	// 创建竞技场服务
	arenaSvc := game.NewArenaService(rdb, userSvc, lockSvc)

	return &Container{
		UserService:     userSvc,
		AllianceService: allianceSvc,
		BuildingService: buildingSvc,
		TroopsService:   troopsSvc,
		MarchService:    marchSvc,
		Hub:             hub,
		ServerService:   serverSvc,
		MapService:      mapSvc,
		PlayerService:   playerSvc,
		ArenaService:    arenaSvc,
	}
}
