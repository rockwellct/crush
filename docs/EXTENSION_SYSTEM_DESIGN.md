# Charm Crush Dynamic Extension System Architecture

This document specifies the architectural design for adding a **runtime extension engine** to Charm Crush, enabling users and plugin authors to customize the agent harness, register custom tools, intercept lifecycle events, and inject custom Bubble Tea UI components without recompiling Crush.

---

## 1. Executive Summary & Goals

### Current State
Charm Crush has native agent loops, tool handlers, and Bubble Tea TUI components, but lacks a plugin/extension architecture for user customization at runtime.

### Design Goals
1. **Dynamic Runtime Loading**: Load extensions automatically from `~/.config/crush/plugins/` and workspace-local `.crush/plugins/` without modifying or recompiling the core Crush binary.
2. **Dual-Language Provider Model**:
   - **Lua (`gopher-lua`)**: Lightweight, battle-tested for quick configuration, lifecycle hooks, and simple tool definitions (similar to Neovim and WezTerm).
   - **Go (`yaegi`)**: Interpreted pure Go for deep integration, complex logic, and native `tea.Model` / `lipgloss.Style` UI components.
3. **Extensibility Surface**:
   - **Tools**: Register custom LLM-callable tools with parameter schemas.
   - **Lifecycle Hooks**: Intercept and modify prompts (`on_input`), gate destructive operations (`on_tool_call`), and track telemetry (`on_message`).
   - **UI & Keybindings**: Custom Bubble Tea widgets, header/footer indicators, custom slash commands, and modal overlays.

---

## 2. System Architecture

```
                                  ┌─────────────────────────────────────────────────────────┐
                                  │                    Charm Crush Core                     │
                                  │        (Bubble Tea App + Agent Loop + Session)          │
                                  └────────────────────────────┬────────────────────────────┘
                                                               │
                                  ┌────────────────────────────▼────────────────────────────┐
                                  │                 Extension Host Engine                   │
                                  │            (Discovery, EventBus, Registries)            │
                                  └───────────────┬─────────────────────────┬───────────────┘
                                                  │                         │
                               ┌──────────────────┴──────────┐   ┌──────────┴──────────────────┐
                               │        Lua Provider         │   │         Go Provider         │
                               │        (gopher-lua)         │   │           (Yaegi)           │
                               ├─────────────────────────────┤   ├─────────────────────────────┤
                               │ • Interprets `*.lua`        │   │ • Interprets `*.go`         │
                               │ • Pure Go (zero CGo)        │   │ • Pure Go (zero CGo)        │
                               │ • Idiomatic table API       │   │ • Real `tea.Model` / types  │
                               └──────────────┬──────────────┘   └──────────┬──────────────────┘
                                              │                             │
                                              ▼                             ▼
                                  ~/.config/crush/plugins/      ~/.config/crush/plugins/
                                     ├── safety.lua                ├── todo_widget.go
                                     └── slack_notify.lua          └── custom_diff.go
```

---

## 3. Core Host API Surface (`pkg/ext`)

The extension host exposes a unified Go interface that both Lua and Yaegi providers bind to.

```go
package ext

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// API is the primary interface passed to extensions during initialization.
type API interface {
	// Tool Management
	RegisterTool(tool ToolDef) error

	// Lifecycle Events
	On(event string, handler EventHandler)

	// Slash Commands & Keybindings
	RegisterCommand(name string, desc string, handler CommandHandler)
	RegisterKeybinding(key string, desc string, handler KeyHandler)

	// TUI & Widgets
	SetHeaderWidget(factory func() tea.Model)
	SetSidebarWidget(factory func() tea.Model)
	RegisterModal(name string, factory func() tea.Model)

	// Notifications & State
	Notify(message string, level NotificationLevel)
	GetCwd() string
	GetSessionName() string
	GetModel() string
}

// ToolDef defines a custom LLM tool.
type ToolDef struct {
	Name        string                                      `json:"name"`
	Description string                                      `json:"description"`
	Parameters  map[string]interface{}                      `json:"parameters"`
	Execute     func(args map[string]interface{}) (string, error)
	ViewModel   func(result string, expanded bool) tea.Model // Optional custom Bubble Tea view
}

// Lifecycle Event Types
const (
	EventSessionStart   = "session_start"
	EventSessionEnd     = "session_end"
	EventInput          = "input"          // Transform or validate user prompt
	EventToolCall       = "tool_call"      // Intercept / approve tool execution
	EventToolResult     = "tool_result"    // Observe tool output
	EventMessageUpdate  = "message_update" // Observe live assistant streaming
)

// Decision returned by tool_call hooks
type ToolDecision string
const (
	AllowOnce   ToolDecision = "allow_once"
	AlwaysAllow ToolDecision = "always_allow"
	Deny        ToolDecision = "deny"
)
```

---

## 4. Extension Language Providers

### Provider A: Lua (`gopher-lua`)
- **Package**: `github.com/yuin/gopher-lua`
- **Characteristics**: Pure Go, zero CGo, embedded instantly, runs on Linux/macOS/Windows.
- **Lua Environment**: Exposes a global `crush` table.

#### Lua Example: Safety Gate & Custom Tool (`~/.config/crush/plugins/security.lua`)
```lua
-- Block dangerous bash commands automatically
crush.on("tool_call", function(evt)
    if evt.tool == "bash" then
        if string.find(evt.command, "rm -rf /") or string.find(evt.command, "mkfs") then
            crush.notify("Blocked destructive command: " .. evt.command, "error")
            return crush.DENY
        end
    end
    return crush.ALLOW
end)

-- Register a custom deployment tool
crush.register_tool({
    name = "deploy_staging",
    description = "Triggers staging deployment pipeline",
    parameters = {
        type = "object",
        properties = {
            service = { type = "string", description = "Service to deploy" }
        },
        required = { "service" }
    },
    execute = function(args)
        crush.notify("Deploying " .. args.service .. "...", "info")
        -- Call external command or API
        return "Deployment triggered successfully for " .. args.service
    end
})
```

---

### Provider B: Go (`yaegi`)
- **Package**: `github.com/traefik/yaegi/interp`
- **Characteristics**: Pure Go interpreter, interprets standard `.go` source files without compilation, supports standard library and imported host packages.
- **Exported Symbols**: Crush exports `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, and `github.com/charmbracelet/crush/pkg/ext` to the Yaegi interpreter.

#### Go Example: Interactive Todo Widget (`~/.config/crush/plugins/todo.go`)
```go
package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/crush/pkg/ext"
)

type TodoWidget struct {
	todos []string
}

func (m TodoWidget) Init() tea.Cmd { return nil }
func (m TodoWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m TodoWidget) View() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	return style.Render(fmt.Sprintf("📋 Tasks: %d pending", len(m.todos)))
}

func Init(api ext.API) {
	// Register sidebar widget
	api.SetSidebarWidget(func() tea.Model {
		return TodoWidget{todos: []string{"Refactor layout", "Add tests"}}
	})

	// Register slash command
	api.RegisterCommand("todo", "Manage active tasks", func(args []string) error {
		api.Notify("Tasks drawer updated", "info")
		return nil
	})
}
```

---

## 5. Extension Lifecycle & Discovery

```
[Crush Startup]
       │
       ▼
1. Scan plugin directories:
   • ~/.config/crush/plugins/
   • .crush/plugins/ (workspace local)
       │
       ▼
2. Group files by extension:
   • *.lua -> Send to GopherLua Engine
   • *.go  -> Send to Yaegi Interpreter Engine
       │
       ▼
3. Instantiate Plugin Context with Crush Host API
       │
       ▼
4. Execute `Init()` / top-level Lua script
       │
       ▼
5. Register declared Tools with active LLM Model Schema
       │
       ▼
6. Attach Event Handlers to Crush EventBus
       │
       ▼
[Crush TUI Main Loop Starts]
```

---

## 6. Phased Implementation Roadmap

1. **Phase 1: Host ABI & Event Bus (`pkg/ext`)**
   - Implement `EventBus` and `ToolRegistry` in Crush.
   - Hook into Crush agent loop (`pre-tool-execution`, `post-tool-execution`, `input-received`).
2. **Phase 2: Lua Provider Integration**
   - Add `gopher-lua` dependency.
   - Build Lua table binding generator for `API`, `ToolDef`, and event callbacks.
   - Test `.lua` scripts for custom slash commands, tool registration, and permission checks.
3. **Phase 3: Yaegi Go Provider Integration**
   - Add `github.com/traefik/yaegi` dependency.
   - Export symbols for `bubbletea`, `lipgloss`, and `pkg/ext`.
   - Test `.go` scripts for custom Bubble Tea widget models and tools.
4. **Phase 4: TUI Mount Points**
   - Add header, footer, and sidebar extension slots in Crush's main Bubble Tea layout.
   - Allow custom models to be mounted into modals or drawer overlays.

---

## 7. Comparison with Alternative Approaches

| Feature | Dual Engine (Lua + Yaegi) | Go Compiled Plugins (`plugin.so`) | Subprocess IPC (JSON-RPC) |
|---|---|---|---|
| **Language Support** | Lua & Go | Go only | Any language |
| **Compilation Step** | None (Drop-in scripts) | Required (`go build -buildmode=plugin`) | Varies by language |
| **Cross-Platform** | ✅ Linux, macOS, Windows | ❌ Linux / macOS only (no Windows) | ✅ Linux, macOS, Windows |
| **Bubble Tea TUI Integration** | ✅ Native in Go, Strings in Lua | ✅ Native in Go | ⚠️ High latency over IPC |
| **Memory / Performance** | In-process, Fast | In-process, Native | Multi-process, IPC overhead |
| **Toolchain Dependency** | None | Exact Go version & build flag match | External runtime required |
