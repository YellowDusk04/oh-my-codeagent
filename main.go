package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/telemetry/langfuse"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("加载 .env 文件失败:", err)
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

	modelInstance := openai.New(
		os.Getenv("MODEL_ID"),
		openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		openai.WithBaseURL(os.Getenv("BASE_URL")),
	)

	agent := llmagent.New("code-agent",
		llmagent.WithModel(modelInstance),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			Stream:          true,
			ThinkingEnabled: new(os.Getenv("ENABLE_THINKING") == "true"),
		}),
	)

	runner := runner.NewRunner("oh-my-codeagent", agent)

	// 设置 Langfuse baggage（让框架自动创建的 span 关联到正确的 user/session）
	userID := "user-001"
	sessionID := fmt.Sprintf("session-%d", time.Now().Unix())
	ctx := NewBaggageContext(userID, sessionID)

	events, err := runner.Run(ctx,
		userID,
		sessionID,
		model.NewUserMessage("你是谁呀?"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Process event stream.
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
