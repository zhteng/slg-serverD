package main

/*
跨服入口（引用 auth.AccessTokenAuth()）
*/
import (
	"log"
	"slg-serverD/auth"
	"slg-serverD/conf"
	"slg-serverD/game"

	"github.com/gin-gonic/gin"
)

func main() {
	crossCfg, err := conf.LoadCrossConfig("cross_config.yaml")
	if err != nil {
		log.Fatalf("load cross config: %v", err)
	}

	router := gin.Default()
	// 跨服活动接口，使用 JWT 验证（使用原服完全相同的鉴权中间件）
	router.Use(auth.AccessTokenAuth())

	crossSvc := game.NewCrossService(crossCfg)
	router.POST("/cross/attack", crossSvc.HandleCrossAttack)
}
