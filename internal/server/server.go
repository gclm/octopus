package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gclm/octopus/internal/conf"
	"github.com/gclm/octopus/internal/relay/bodycache"
	_ "github.com/gclm/octopus/internal/server/handlers"
	"github.com/gclm/octopus/internal/server/middleware"
	"github.com/gclm/octopus/internal/server/resp"
	"github.com/gclm/octopus/internal/server/router"
	"github.com/gclm/octopus/internal/utils/log"
	"github.com/gclm/octopus/static"
	"github.com/gin-gonic/gin"
)

var httpSrv http.Server

func Start() error {
	if conf.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 启动时清理 Images 请求体临时文件（失败仅告警，不阻断启动）
	tmpDir := bodycache.TmpDirFromEnv()
	olderThan := bodycache.TmpCleanupOlderThanFromEnv()
	if err := bodycache.CleanupOldTmpFiles(tmpDir, bodycache.TmpFilePrefix, olderThan); err != nil {
		log.Warnf("cleanup images tmp files failed: dir=%s prefix=%s olderThan=%s err=%v", tmpDir, bodycache.TmpFilePrefix, olderThan, err)
	}

	r := gin.New()
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		resp.Error(c, http.StatusInternalServerError, resp.ErrInternalServer)
		c.Abort()
	}))

	if conf.IsDebug() {
		r.Use(middleware.Logger())
	}
	r.Use(middleware.Cors())
	r.Use(middleware.StaticEmbed("/", static.StaticFS))

	router.RegisterAll(r)

	httpSrv.Addr = fmt.Sprintf("%s:%d", conf.AppConfig.Server.Host, conf.AppConfig.Server.Port)
	httpSrv.Handler = r
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("http server listen and serve error: %v", err)
		}
	}()
	return nil
}

func Close() error {
	// 优雅关闭：等待正在进行的请求完成，最多等待 30 秒
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	log.Infof("shutting down HTTP server gracefully...")
	if err := httpSrv.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server forced to shutdown: %w", err)
	}
	log.Infof("HTTP server shutdown complete")
	return nil
}
