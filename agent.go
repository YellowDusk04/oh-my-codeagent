package main

import (
	"os"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

const (
	agentName = "code-agent"
	appName   = "oh-my-codeagent"
)

// createRunner 创建 runner，包含 agent 和 session service
func createRunner(sessionService session.Service) (runner.Runner, error) {
	modelInstance := openai.New(
		os.Getenv("MODEL_ID"),
		openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		openai.WithBaseURL(os.Getenv("BASE_URL")),
	)

	enableThinking := os.Getenv("ENABLE_THINKING") == "true"

	agent := llmagent.New(agentName,
		llmagent.WithModel(modelInstance),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			Stream:          true,
			ThinkingEnabled: &enableThinking,
		}),
	)

	r := runner.NewRunner(appName, agent,
		runner.WithSessionService(sessionService),
	)

	return r, nil
}
