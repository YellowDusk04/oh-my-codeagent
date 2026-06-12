package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

// createSessionService 创建 SQLite session service
func createSessionService() (session.Service, error) {
	// SQLite DSN，可以根据需要修改
	// 也可以通过环境变量配置
	dsn := "file:sessions.db?_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 建议单连接
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	svc, err := sqlite.NewService(
		db,
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("创建 session service 失败: %w", err)
	}

	return svc, nil
}
