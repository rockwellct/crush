package ext

import (
	tea "charm.land/bubbletea/v2"
)

// NotificationLevel represents the severity of a notification.
type NotificationLevel string

const (
	NotifyInfo    NotificationLevel = "info"
	NotifyWarn    NotificationLevel = "warn"
	NotifyError   NotificationLevel = "error"
	NotifySuccess NotificationLevel = "success"
)

// ToolDecision returned by tool_call hooks.
type ToolDecision string

const (
	AllowOnce   ToolDecision = "allow_once"
	AlwaysAllow ToolDecision = "always_allow"
	Deny        ToolDecision = "deny"
)

// Lifecycle Event Types.
const (
	EventSessionStart  = "session_start"
	EventSessionEnd    = "session_end"
	EventInput         = "input"          // Transform or validate user prompt
	EventToolCall      = "tool_call"      // Intercept / approve tool execution
	EventToolResult    = "tool_result"    // Observe tool output
	EventMessageUpdate = "message_update" // Observe live assistant streaming
)

// Event is the payload passed to event handlers.
type Event struct {
	Name      string         `json:"name"`
	SessionID string         `json:"session_id,omitempty"`
	Prompt    string         `json:"prompt,omitempty"`
	Tool      string         `json:"tool,omitempty"`
	Command   string         `json:"command,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Result    string         `json:"result,omitempty"`
	Message   string         `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// EventHandler handles lifecycle events and may return a modified payload, decision, or error.
type EventHandler func(evt Event) (any, error)

// CommandHandler handles custom slash command invocations.
type CommandHandler func(args []string) error

// KeyHandler handles custom keybinding presses.
type KeyHandler func() error

// WidgetFactory returns a Bubble Tea model for mounting in the UI.
type WidgetFactory func() tea.Model

// ToolDef defines a custom LLM-callable tool registered by an extension.
type ToolDef struct {
	Name        string                                       `json:"name"`
	Description string                                       `json:"description"`
	Parameters  map[string]any                               `json:"parameters"`
	Execute     func(args map[string]any) (string, error)    `json:"-"`
	ViewModel   func(result string, expanded bool) tea.Model `json:"-"`
}

// SlashCommand represents a registered slash command.
type SlashCommand struct {
	Name        string
	Description string
	Handler     CommandHandler
}

// Keybinding represents a registered custom keybinding.
type Keybinding struct {
	Key         string
	Description string
	Handler     KeyHandler
}

// API is the primary host interface exposed to extensions during initialization.
type API interface {
	// Tool Management
	RegisterTool(tool ToolDef) error

	// Lifecycle Events
	On(event string, handler EventHandler)

	// Slash Commands & Keybindings
	RegisterCommand(name string, desc string, handler CommandHandler)
	RegisterKeybinding(key string, desc string, handler KeyHandler)

	// TUI & Widgets
	SetHeaderWidget(factory WidgetFactory)
	SetSidebarWidget(factory WidgetFactory)
	RegisterModal(name string, factory WidgetFactory)

	// Notifications & State
	Notify(message string, level NotificationLevel)
	GetCwd() string
	GetSessionName() string
	GetModel() string
}
