package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("加载 .env 文件失败:", err)
	}

	modelInstance := openai.New(os.Getenv("MODEL_ID"),
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

	events, err := runner.Run(context.Background(),
		"user-001",
		"session-001",
		model.NewUserMessage("你好"),
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
