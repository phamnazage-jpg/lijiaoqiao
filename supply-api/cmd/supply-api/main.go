package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lijiaoqiao/supply-api/internal/app"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

func main() {
	// 解析命令行参数
	env := flag.String("env", "dev", "environment: dev/staging/prod")
	configPath := flag.String("config", "", "config file path")
	flag.Parse()

	envName, err := app.ResolveEnv(*env)
	if err != nil {
		logging.NewLogger("supply-api", logging.LogLevelInfo).Fatalf("%v", err)
	}
	*env = envName

	// 确定配置文件路径
	if *configPath == "" {
		*configPath = "./config/config." + *env + ".yaml"
	}
	if err := validateEnvConfigPath(*env, *configPath); err != nil {
		logging.NewLogger("supply-api", logging.LogLevelInfo).Fatalf("%v", err)
	}

	// P1-010修复: 初始化结构化日志
	jsonLogger := logging.NewLogger("supply-api", logging.LogLevelInfo)

	// 加载配置
	cfg, err := config.LoadFromPath(*env, *configPath)
	if err != nil {
		jsonLogger.Fatalf("failed to load config: %v", err)
	}

	jsonLogger.Infof("starting supply-api in %s mode", *env)
	rootCtx, stop := context.WithCancel(context.Background())
	defer stop()

	initCtx, initCancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer initCancel()

	runtime, err := app.BuildRuntime(app.RuntimeOptions{
		Env:         *env,
		Config:      cfg,
		Logger:      jsonLogger,
		InitContext: initCtx,
		Now:         time.Now,
	})
	if err != nil {
		if *env == "prod" {
			jsonLogger.Fatalf("production startup requirement failed: %v", err)
		}
		jsonLogger.Fatalf("failed to build runtime: %v", err)
	}
	defer runtime.Close()

	srv, err := runtime.BuildServer()
	if err != nil {
		jsonLogger.Fatalf("failed to build http server: %v", err)
	}

	serverErrCh := make(chan error, 1)
	go func() {
		jsonLogger.Infof("starting HTTP server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	if err := runtime.StartBackgroundWorkers(rootCtx, initCtx); err != nil {
		if *env == "prod" {
			jsonLogger.Fatalf("production startup requirement failed: %v", err)
		}
		jsonLogger.Fatalf("failed to start background workers: %v", err)
	}

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		jsonLogger.Infof("received signal %s", sig)
	case err := <-serverErrCh:
		jsonLogger.Fatalf("server failed: %v", err)
	}

	stop()

	jsonLogger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), runtime.ShutdownTimeout())
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		jsonLogger.Infof("graceful shutdown failed: %v", err)
	}

	jsonLogger.Info("shutdown complete")
}

func validateEnvConfigPath(envName, configPath string) error {
	if envName == "dev" {
		return nil
	}

	base := strings.ToLower(strings.TrimSpace(filepath.Base(configPath)))
	switch base {
	case "config.dev.yaml", "config.dev.yml":
		return fmt.Errorf(
			"config path %q is a dev template and cannot be used with -env=%s; use config.%s.yaml or a %s example template instead",
			configPath,
			envName,
			envName,
			envName,
		)
	default:
		return nil
	}
}
