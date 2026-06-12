package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/telemetry/langfuse"
)

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	// 启动 Langfuse 遥测
	clean, err := langfuse.Start(context.Background())
	if err != nil {
		log.Printf("Failed to start Langfuse telemetry: %v", err)
	} else {
		defer func() {
			if err := clean(context.Background()); err != nil {
				log.Printf("Failed to clean up Langfuse telemetry: %v", err)
			}
		}()
	}

	// 创建 session service
	sessionService, err := createSessionService()
	if err != nil {
		log.Fatal("创建 session service 失败:", err)
	}

	// 创建 runner（包含 agent）
	r, err := createRunner(sessionService)
	if err != nil {
		log.Fatal("创建 runner 失败:", err)
	}

	// 设置用户和会话 ID
	userID := "user-001"
	sessionID := fmt.Sprintf("session-%d", time.Now().Unix())

	// 启动多轮对话
	startChat(r, userID, &sessionID)

	// 退出时打印 session ID
	fmt.Printf("\n📝 当前 session ID: %s\n", sessionID)
	fmt.Println("💡 下次启动时可以使用此 ID 恢复会话")
}

// startChat 启动多轮对话循环
func startChat(r runner.Runner, userID string, sessionID *string) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("🤖 多轮对话已启动！\n")
	fmt.Printf("📝 当前 session ID: %s\n", *sessionID)
	fmt.Println("💡 命令: /exit 退出, /session new 新建会话, /session <id> 切换会话")

	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}

		userInput := scanner.Text()
		if userInput == "" {
			continue
		}

		// 退出命令
		if userInput == "/exit" {
			fmt.Println("👋 再见！")
			break
		}

		// 处理 /session 命令
		if strings.HasPrefix(userInput, "/session") {
			handleSessionCommand(userInput, sessionID)
			continue
		}

		// 创建带 baggage 的 context
		ctx := NewBaggageContext(userID, *sessionID)

		events, err := r.Run(ctx,
			userID,
			*sessionID,
			model.NewUserMessage(userInput),
		)
		if err != nil {
			log.Println("运行失败:", err)
			continue
		}

		// 处理事件流
		fmt.Print("Assistant: ")
		for event := range events {
			if event != nil && len(event.Choices) > 0 {
				choice := event.Choices[0]
				if choice.Delta.Content != "" {
					fmt.Print(choice.Delta.Content)
				}
			}
		}
		fmt.Println()
	}
}

// handleSessionCommand 处理 /session 命令
func handleSessionCommand(input string, sessionID *string) {
	parts := strings.Fields(input)

	if len(parts) == 1 {
		// /session - 显示当前 session ID
		fmt.Printf("📝 当前 session ID: %s\n", *sessionID)
		return
	}

	if len(parts) == 2 && parts[1] == "new" {
		// /session new - 创建新会话
		*sessionID = fmt.Sprintf("session-%d", time.Now().Unix())
		fmt.Printf("✨ 已创建新会话\n")
		fmt.Printf("📝 新 session ID: %s\n", *sessionID)
		return
	}

	if len(parts) == 2 {
		// /session <id> - 切换到指定会话
		*sessionID = parts[1]
		fmt.Printf("🔄 已切换到会话: %s\n", *sessionID)
		return
	}

	fmt.Println("❌ 用法: /session | /session new | /session <id>")
}
