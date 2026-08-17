package ext

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
)

// NotificationCallback receives notifications sent from extensions.
type NotificationCallback func(message string, level NotificationLevel)

// Manager coordinates extension discovery, lifecycle, tool registry, and event dispatch.
type Manager struct {
	mu          sync.RWMutex
	cwd         string
	sessionName string
	model       string
	onNotify    NotificationCallback

	eventBus      *EventBus
	luaProvider   *LuaProvider
	yaegiProvider *YaegiProvider

	tools       map[string]ToolDef
	agentTools  []fantasy.AgentTool
	commands    map[string]SlashCommand
	keybindings map[string]Keybinding

	headerWidget  WidgetFactory
	sidebarWidget WidgetFactory
	modals        map[string]WidgetFactory
}

// NewManager creates an initialized extension Manager.
func NewManager(cwd string) *Manager {
	return &Manager{
		cwd:           cwd,
		eventBus:      NewEventBus(),
		luaProvider:   NewLuaProvider(),
		yaegiProvider: NewYaegiProvider(),
		tools:         make(map[string]ToolDef),
		commands:      make(map[string]SlashCommand),
		keybindings:   make(map[string]Keybinding),
		modals:        make(map[string]WidgetFactory),
	}
}

// SetNotificationCallback sets the callback function for extension notifications.
func (m *Manager) SetNotificationCallback(cb NotificationCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNotify = cb
}

// SetState updates dynamic state context such as current session or model name.
func (m *Manager) SetState(sessionName, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionName = sessionName
	m.model = model
}

// EventBus returns the extension manager's event bus.
func (m *Manager) EventBus() *EventBus {
	return m.eventBus
}

// AgentTools returns all fantasy.AgentTool wrappers for registered extension tools.
func (m *Manager) AgentTools() []fantasy.AgentTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]fantasy.AgentTool(nil), m.agentTools...)
}

// Commands returns all registered custom slash commands.
func (m *Manager) Commands() map[string]SlashCommand {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make(map[string]SlashCommand, len(m.commands))
	for k, v := range m.commands {
		res[k] = v
	}
	return res
}

// Keybindings returns all registered custom keybindings.
func (m *Manager) Keybindings() map[string]Keybinding {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make(map[string]Keybinding, len(m.keybindings))
	for k, v := range m.keybindings {
		res[k] = v
	}
	return res
}

// HeaderWidget returns a new header widget model instance if registered.
func (m *Manager) HeaderWidget() tea.Model {
	m.mu.RLock()
	factory := m.headerWidget
	m.mu.RUnlock()
	if factory != nil {
		return factory()
	}
	return nil
}

// SidebarWidget returns a new sidebar widget model instance if registered.
func (m *Manager) SidebarWidget() tea.Model {
	m.mu.RLock()
	factory := m.sidebarWidget
	m.mu.RUnlock()
	if factory != nil {
		return factory()
	}
	return nil
}

// Modal returns a new modal widget model instance by name if registered.
func (m *Manager) Modal(name string) tea.Model {
	m.mu.RLock()
	factory := m.modals[name]
	m.mu.RUnlock()
	if factory != nil {
		return factory()
	}
	return nil
}

// DiscoverAndLoad searches default plugin paths and loads all .lua and .go extensions.
func (m *Manager) DiscoverAndLoad(extraDirs ...string) error {
	var searchDirs []string

	// Global user plugins: ~/.config/crush/plugins/
	if configHome, err := os.UserConfigDir(); err == nil {
		searchDirs = append(searchDirs, filepath.Join(configHome, "crush", "plugins"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		searchDirs = append(searchDirs, filepath.Join(home, ".config", "crush", "plugins"))
	}

	// Workspace local plugins: .crush/plugins/
	if m.cwd != "" {
		searchDirs = append(searchDirs, filepath.Join(m.cwd, ".crush", "plugins"))
	}

	searchDirs = append(searchDirs, extraDirs...)

	seenDirs := make(map[string]bool)
	for _, dir := range searchDirs {
		clean := filepath.Clean(dir)
		if seenDirs[clean] {
			continue
		}
		seenDirs[clean] = true

		if err := m.LoadPluginsFromDir(clean); err != nil {
			slog.Warn("Failed scanning extension directory", "dir", clean, "error", err)
		}
	}

	return nil
}

// LoadPluginsFromDir loads all plugins from a specific directory.
func (m *Manager) LoadPluginsFromDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))

		switch ext {
		case ".lua":
			slog.Info("Loading Lua extension", "path", path)
			if err := m.luaProvider.LoadFile(path, m); err != nil {
				slog.Error("Failed loading Lua extension", "path", path, "error", err)
			}
		case ".go":
			slog.Info("Loading Go extension", "path", path)
			if err := m.yaegiProvider.LoadFile(path, m); err != nil {
				slog.Error("Failed loading Go extension", "path", path, "error", err)
			}
		}
	}

	return nil
}

// --- ext.API Interface Implementation ---

// RegisterTool registers an LLM tool with the manager.
func (m *Manager) RegisterTool(tool ToolDef) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	m.tools[tool.Name] = tool
	agentTool := NewExtensionAgentTool(tool)

	// Replace existing tool if duplicate name
	replaced := false
	for i, existing := range m.agentTools {
		if existing.Info().Name == tool.Name {
			m.agentTools[i] = agentTool
			replaced = true
			break
		}
	}
	if !replaced {
		m.agentTools = append(m.agentTools, agentTool)
	}

	return nil
}

// On subscribes a handler to a lifecycle event.
func (m *Manager) On(event string, handler EventHandler) {
	m.eventBus.Subscribe(event, handler)
}

// RegisterCommand registers a slash command.
func (m *Manager) RegisterCommand(name string, desc string, handler CommandHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands[name] = SlashCommand{
		Name:        name,
		Description: desc,
		Handler:     handler,
	}
}

// RegisterKeybinding registers a keybinding.
func (m *Manager) RegisterKeybinding(key string, desc string, handler KeyHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keybindings[key] = Keybinding{
		Key:         key,
		Description: desc,
		Handler:     handler,
	}
}

// SetHeaderWidget registers a header widget factory.
func (m *Manager) SetHeaderWidget(factory WidgetFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.headerWidget = factory
}

// SetSidebarWidget registers a sidebar widget factory.
func (m *Manager) SetSidebarWidget(factory WidgetFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sidebarWidget = factory
}

// RegisterModal registers a modal overlay widget factory.
func (m *Manager) RegisterModal(name string, factory WidgetFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modals[name] = factory
}

// Notify triggers an extension notification.
func (m *Manager) Notify(message string, level NotificationLevel) {
	m.mu.RLock()
	cb := m.onNotify
	m.mu.RUnlock()

	if cb != nil {
		cb(message, level)
	} else {
		slog.Info("Extension notification", "level", level, "message", message)
	}
}

// GetCwd returns current working directory.
func (m *Manager) GetCwd() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cwd
}

// GetSessionName returns current session name.
func (m *Manager) GetSessionName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionName
}

// GetModel returns current active model.
func (m *Manager) GetModel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.model
}
