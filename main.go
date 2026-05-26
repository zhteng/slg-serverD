package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slg-serverD/cache"
	"strconv"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"slg-serverD/api"
	"slg-serverD/conf"
	"slg-serverD/di"
)

func initLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

func buildRuntimeConfig(cfg *conf.Config, serverID int) *conf.RuntimeConfig {
	// 如果未找到指定区服配置，回退到全局配置（默认）
	rtCfg := &conf.RuntimeConfig{
		ServerID:       serverID,
		DBDSN:          cfg.DB.DSN,
		HTTPPort:       cfg.Server.HTTPPort,
		RedisAddr:      cfg.Redis.Addr,
		RedisPass:      cfg.Redis.Password,
		RedisPrefix:    "",
		Status:         1,
		MaxPlayers:     20000,
		InternalSecret: cfg.InternalSecret,
	}

	for _, svr := range cfg.Servers {
		if svr.ID == serverID {
			rtCfg.ServerName = svr.Name
			if svr.DB.DSN != "" {
				rtCfg.DBDSN = svr.DB.DSN
			}

			if svr.Redis.Addr != "" {
				rtCfg.RedisAddr = svr.Redis.Addr
			}
			rtCfg.RedisPass = svr.Redis.Password
			rtCfg.RedisPrefix = svr.Redis.Prefix
			if svr.HTTPPort != 0 {
				rtCfg.HTTPPort = svr.HTTPPort
			}
			break
		}
	}

	return rtCfg
}

func getServerID() int {
	if env := os.Getenv("SERVER_ID"); env != "" {
		id, err := strconv.Atoi(env)
		if err == nil {
			return id
		}
	}

	return 1
}

func main() {
	/*err := godotenv.Load()
	if err != nil {
		fmt.Println("加载 .env 文件失败:", err)
	}

	serverPort := os.Getenv("JWT_ACCESS_SECRET")

	fmt.Println("Server port:", serverPort)
	os.Exit(1)*/

	// 加载配置
	cfg, err := conf.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 2. 获取当前要启动的区服 ID
	serverID := getServerID()

	// 构建运行时配置
	rtCfg := buildRuntimeConfig(cfg, serverID)

	if rtCfg.InternalSecret == "" {
		rtCfg.InternalSecret = os.Getenv("INTERNAL_SECRET")
	}

	if rtCfg.InternalSecret == "" {
		log.Fatal("INTERNAL_SECRET not set! ...............")
	}

	conf.SetRuntimeConfig(rtCfg)
	log.Printf("Starting server: ID=%d, Name=%s, Port=%d", rtCfg.ServerID, rtCfg.ServerName, rtCfg.HTTPPort)

	// 加载建筑配置
	buildingConf, err := conf.LoadBuildingConfig("building_config.yaml")
	if err != nil {
		log.Fatalf("Failed to load building config: %v", err)
	}

	// 加载部队配置
	troopsConf, err := conf.LoadTroopsConfig("troops_config.yaml")
	if err != nil {
		log.Fatalf("Failed to load troops config: %v", err)
	}

	// 初始化数据库
	dbConn, err := sql.Open("mysql", rtCfg.DBDSN)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := dbConn.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	dbConn.SetMaxOpenConns(20) // 连接池设置
	dbConn.SetMaxIdleConns(10)
	dbConn.SetConnMaxLifetime(time.Hour)
	log.Println("Database connected successfully...")

	// 初始化 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     rtCfg.RedisAddr,
		Password: rtCfg.RedisPass,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("ping redis: %v", err)
	}
	// 6. 设置 Redis 全局前缀（用于逻辑隔离）
	cache.SetGlobalPrefix(rtCfg.RedisPrefix)

	log.Println("Redis connected successfully...")

	//mapCfg, _ := conf.LoadMapConfig("config/map_config.yaml")
	//mapCache := cache.NewMapCache(rdb, mapCfg)

	// 通过依赖注入容器构建所有服务
	ctn := di.NewContainer(dbConn, rdb, cfg, rtCfg, buildingConf, troopsConf)

	// ------------------------------------  启动后台任务（延迟队列消费者） ------------------------------------
	// 启动建筑队列定时扫描（每2秒一次）
	//ctn.BuildingService.StartQueueChecker(2 * time.Second)

	// 启动部队队列定时扫描
	ctn.TroopsService.StartQueueChecker()

	// 行军
	ctn.MarchService.StartQueueChecker()

	// 初始化地图
	ctn.MapService.InitializeMap()

	// 构建 Gin 路由
	apiServer := api.NewServer(
		ctn.UserService,
		ctn.AllianceService,
		ctn.BuildingService,
		ctn.TroopsService,
		ctn.MarchService,
		ctn.Hub,
		ctn.ServerService,
		ctn.MapService,
		ctn.PlayerService,
		ctn.ArenaService,
		rtCfg.InternalSecret,
	)
	router := apiServer.SetupRouter()

	// 创建 HTTP Server（支持优雅关闭）
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", rtCfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

	// 启动服务器（非阻塞）
	go func() {
		log.Printf("SLG server started on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server ...")

	// 停止后台所有服务
	//ctn.BuildingService.Stop()
	ctn.TroopsService.Stop()
	ctn.MarchService.Stop()
	ctn.MapService.Stop()

	// 给服务 5 秒处理剩余请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited gracefully")

	/*
		// 原生的 http.ListenAndServe
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)
		log.Printf("SLG server started on :%d", cfg.Server.HTTPPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Server.HTTPPort), mux))
	*/

}
