package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	addr      string
	database  string
	selfcheck bool
	timeout   time.Duration
}

func defaultAddress() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return "127.0.0.1:19081"
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return "127.0.0.1:19081"
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(value))
}

func validateAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("监听地址格式错误: %w", err)
	}
	parsed := net.ParseIP(host)
	if parsed == nil || !parsed.IsLoopback() {
		return errors.New("监听地址必须使用显式回环 IP")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return errors.New("监听端口必须在 1 到 65535 之间")
	}
	return nil
}
