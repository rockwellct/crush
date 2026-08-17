package ext_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/pkg/ext"
	"github.com/stretchr/testify/require"
)

func TestEventBus(t *testing.T) {
	t.Parallel()

	bus := ext.NewEventBus()

	// Test prompt modification on input event
	bus.Subscribe(ext.EventInput, func(evt ext.Event) (any, error) {
		return evt.Prompt + " [modified]", nil
	})

	res, err := bus.Emit(ext.Event{
		Name:   ext.EventInput,
		Prompt: "hello",
	})
	require.NoError(t, err)
	require.Equal(t, "hello [modified]", res)

	// Test tool call deny decision
	bus.Subscribe(ext.EventToolCall, func(evt ext.Event) (any, error) {
		if evt.Command == "rm -rf /" {
			return ext.Deny, nil
		}
		return ext.AllowOnce, nil
	})

	res, err = bus.Emit(ext.Event{
		Name:    ext.EventToolCall,
		Command: "rm -rf /",
	})
	require.NoError(t, err)
	require.Equal(t, ext.Deny, res)

	res, err = bus.Emit(ext.Event{
		Name:    ext.EventToolCall,
		Command: "ls -la",
	})
	require.NoError(t, err)
	require.Equal(t, ext.AllowOnce, res)
}

func TestToolAdapter(t *testing.T) {
	t.Parallel()

	toolDef := ext.ToolDef{
		Name:        "calculator",
		Description: "Simple calculator",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expr": map[string]any{
					"type":        "string",
					"description": "Expression to evaluate",
				},
			},
			"required": []string{"expr"},
		},
		Execute: func(args map[string]any) (string, error) {
			expr, _ := args["expr"].(string)
			return "Result for " + expr, nil
		},
	}

	agentTool := ext.NewExtensionAgentTool(toolDef)
	info := agentTool.Info()
	require.Equal(t, "calculator", info.Name)
	require.Equal(t, "Simple calculator", info.Description)
	require.Contains(t, info.Required, "expr")

	resp, err := agentTool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call_1",
		Name:  "calculator",
		Input: `{"expr":"2+2"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "Result for 2+2", resp.Content)
}

func TestLuaProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	luaScript := `
crush.on("tool_call", function(evt)
    if evt.command == "dangerous_cmd" then
        crush.notify("Blocked dangerous command", "error")
        return crush.DENY
    end
    return crush.ALLOW
end)

crush.register_tool({
    name = "deploy_staging",
    description = "Triggers staging deployment",
    execute = function(args)
        return "Deployed " .. (args.service or "default")
    end
})

crush.register_command("ping", "Ping command", function(args)
    crush.notify("pong", "info")
end)
`
	scriptPath := filepath.Join(dir, "test.lua")
	err := os.WriteFile(scriptPath, []byte(luaScript), 0o644)
	require.NoError(t, err)

	mgr := ext.NewManager(dir)
	var notifications []string
	mgr.SetNotificationCallback(func(message string, level ext.NotificationLevel) {
		notifications = append(notifications, string(level)+": "+message)
	})

	err = mgr.DiscoverAndLoad(dir)
	require.NoError(t, err)

	// Check tool registered
	tools := mgr.AgentTools()
	require.Len(t, tools, 1)
	require.Equal(t, "deploy_staging", tools[0].Info().Name)

	resp, err := tools[0].Run(context.Background(), fantasy.ToolCall{
		ID:    "call_1",
		Name:  "deploy_staging",
		Input: `{"service":"auth-backend"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "Deployed auth-backend", resp.Content)

	// Check command registered
	commands := mgr.Commands()
	cmd, ok := commands["ping"]
	require.True(t, ok)
	err = cmd.Handler([]string{})
	require.NoError(t, err)
	require.Contains(t, notifications, "info: pong")

	// Check event hook blocking
	res, err := mgr.EventBus().Emit(ext.Event{
		Name:    ext.EventToolCall,
		Command: "dangerous_cmd",
	})
	require.NoError(t, err)
	require.Equal(t, ext.Deny, res)
	require.Contains(t, notifications, "error: Blocked dangerous command")
}

func TestYaegiProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goScript := `
package main

import (
	"github.com/charmbracelet/crush/pkg/ext"
)

func Init(api ext.API) {
	api.RegisterTool(ext.ToolDef{
		Name:        "yaegi_tool",
		Description: "Tool from Yaegi Go plugin",
		Execute: func(args map[string]any) (string, error) {
			return "hello from yaegi", nil
		},
	})

	api.RegisterCommand("greet", "Greet user", func(args []string) error {
		api.Notify("Hello from Go extension!", ext.NotifySuccess)
		return nil
	})
}
`
	scriptPath := filepath.Join(dir, "plugin.go")
	err := os.WriteFile(scriptPath, []byte(goScript), 0o644)
	require.NoError(t, err)

	mgr := ext.NewManager(dir)
	var notifications []string
	mgr.SetNotificationCallback(func(message string, level ext.NotificationLevel) {
		notifications = append(notifications, string(level)+": "+message)
	})

	err = mgr.DiscoverAndLoad(dir)
	require.NoError(t, err)

	// Check tool registered
	tools := mgr.AgentTools()
	require.Len(t, tools, 1)
	require.Equal(t, "yaegi_tool", tools[0].Info().Name)

	resp, err := tools[0].Run(context.Background(), fantasy.ToolCall{
		ID:   "call_2",
		Name: "yaegi_tool",
	})
	require.NoError(t, err)
	require.Equal(t, "hello from yaegi", resp.Content)

	// Check command registered
	commands := mgr.Commands()
	cmd, ok := commands["greet"]
	require.True(t, ok)
	err = cmd.Handler([]string{})
	require.NoError(t, err)
	require.Contains(t, notifications, "success: Hello from Go extension!")
}
