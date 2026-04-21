package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lijiaoqiao/gateway/internal/app"
	"lijiaoqiao/gateway/internal/config"
	"lijiaoqiao/gateway/internal/pkg/logging"
)

func main() {
	logger := logging.NewLogger("gateway", logging.LogLevelInfo)

	// 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		logger.Fatalf("failed to load config: %v", err)
	}

	bundle, err := app.BuildServer(cfg)
	if err != nil {
		logger.Fatalf("failed to build server: %v", err)
	}

	serverErrCh := make(chan error, 1)

	// 启动Server
	go func() {
		logger.Infof("starting gateway server on %s", bundle.Server.Addr)
		if err := bundle.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		logger.Infof("received signal %s", sig)
	case err := <-serverErrCh:
		logger.Fatalf("server failed: %v", err)
	}

	logger.Info("shutting down server...")

	// P3-B-06: 停止后台健康检查器
	bundle.ShutdownFunc()

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := bundle.Server.Shutdown(ctx); err != nil {
		logger.Fatalf("server forced to shutdown: %v", err)
	}

	logger.Info("server exited")
}
