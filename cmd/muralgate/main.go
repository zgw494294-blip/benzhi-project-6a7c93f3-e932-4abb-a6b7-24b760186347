package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mural-conservation-gate/internal/httpapi"
	"mural-conservation-gate/internal/store"
	"mural-conservation-gate/internal/workflow"
)

func main() {
	cfg := config{}
	flag.StringVar(&cfg.addr, "addr", defaultAddress(), "HTTP 回环监听地址")
	flag.StringVar(&cfg.database, "db", "muralgate.db", "SQLite 数据库文件")
	flag.BoolVar(&cfg.selfcheck, "selfcheck", false, "启动真实 HTTP 监听并执行完整业务自检后退出")
	flag.DurationVar(&cfg.timeout, "timeout", 15*time.Second, "自检或优雅关闭超时")
	flag.Parse()
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config) error {
	if err := validateAddress(cfg.addr); err != nil {
		return err
	}
	if cfg.timeout <= 0 {
		return fmt.Errorf("timeout 必须为正数")
	}
	database := cfg.database
	if cfg.selfcheck {
		database = ":memory:"
	}
	repository, err := store.Open(database)
	if err != nil {
		return err
	}
	defer repository.Close()
	service := workflow.New(repository)
	api := httpapi.New(service)
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	if cfg.selfcheck {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
		defer cancel()
		checkErr := runSelfcheck(ctx, "http://"+listener.Addr().String())
		shutdownErr := server.Shutdown(context.Background())
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		fmt.Println("自检通过：完整许可流程、验真和审计时间线均可达")
		return nil
	}
	log.Printf("壁画清洗许可门监听于 http://%s", listener.Addr().String())
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err = <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
