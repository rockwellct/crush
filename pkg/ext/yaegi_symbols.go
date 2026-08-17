package ext

import (
	"reflect"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/traefik/yaegi/interp"
)

// Symbols provides Yaegi interpreter symbols for Crush extension APIs, Bubble Tea, and Lipgloss.
var Symbols = map[string]map[string]reflect.Value{}

func init() {
	Symbols["github.com/charmbracelet/crush/pkg/ext/ext"] = map[string]reflect.Value{
		// Types & Interfaces
		"API":               reflect.ValueOf((*API)(nil)),
		"ToolDef":           reflect.ValueOf((*ToolDef)(nil)),
		"SlashCommand":      reflect.ValueOf((*SlashCommand)(nil)),
		"Keybinding":        reflect.ValueOf((*Keybinding)(nil)),
		"Event":             reflect.ValueOf((*Event)(nil)),
		"EventHandler":      reflect.ValueOf((*EventHandler)(nil)),
		"CommandHandler":    reflect.ValueOf((*CommandHandler)(nil)),
		"KeyHandler":        reflect.ValueOf((*KeyHandler)(nil)),
		"WidgetFactory":     reflect.ValueOf((*WidgetFactory)(nil)),
		"ToolDecision":      reflect.ValueOf((*ToolDecision)(nil)),
		"NotificationLevel": reflect.ValueOf((*NotificationLevel)(nil)),

		// Constants
		"AllowOnce":          reflect.ValueOf(AllowOnce),
		"AlwaysAllow":        reflect.ValueOf(AlwaysAllow),
		"Deny":               reflect.ValueOf(Deny),
		"NotifyInfo":         reflect.ValueOf(NotifyInfo),
		"NotifyWarn":         reflect.ValueOf(NotifyWarn),
		"NotifyError":        reflect.ValueOf(NotifyError),
		"NotifySuccess":      reflect.ValueOf(NotifySuccess),
		"EventSessionStart":  reflect.ValueOf(EventSessionStart),
		"EventSessionEnd":    reflect.ValueOf(EventSessionEnd),
		"EventInput":         reflect.ValueOf(EventInput),
		"EventToolCall":      reflect.ValueOf(EventToolCall),
		"EventToolResult":    reflect.ValueOf(EventToolResult),
		"EventMessageUpdate": reflect.ValueOf(EventMessageUpdate),
	}

	// Alias for direct package import `pkg/ext`
	Symbols["pkg/ext/ext"] = Symbols["github.com/charmbracelet/crush/pkg/ext/ext"]

	// Export Bubble Tea v2 symbols
	Symbols["charm.land/bubbletea/v2/tea"] = map[string]reflect.Value{
		"Model":    reflect.ValueOf((*tea.Model)(nil)),
		"Cmd":      reflect.ValueOf((*tea.Cmd)(nil)),
		"Msg":      reflect.ValueOf((*tea.Msg)(nil)),
		"Batch":    reflect.ValueOf(tea.Batch),
		"Sequence": reflect.ValueOf(tea.Sequence),
		"Quit":     reflect.ValueOf(tea.Quit),
	}
	Symbols["github.com/charmbracelet/bubbletea/tea"] = Symbols["charm.land/bubbletea/v2/tea"]

	// Export Lipgloss v2 symbols
	Symbols["charm.land/lipgloss/v2/lipgloss"] = map[string]reflect.Value{
		"NewStyle": reflect.ValueOf(lipgloss.NewStyle),
		"Color":    reflect.ValueOf(lipgloss.Color),
		"Style":    reflect.ValueOf((*lipgloss.Style)(nil)),
		"Left":     reflect.ValueOf(lipgloss.Left),
		"Center":   reflect.ValueOf(lipgloss.Center),
		"Right":    reflect.ValueOf(lipgloss.Right),
		"Top":      reflect.ValueOf(lipgloss.Top),
		"Bottom":   reflect.ValueOf(lipgloss.Bottom),
	}
	Symbols["github.com/charmbracelet/lipgloss/lipgloss"] = Symbols["charm.land/lipgloss/v2/lipgloss"]
}

// ExportCustomSymbols registers symbols into a Yaegi interpreter instance.
func ExportCustomSymbols(i *interp.Interpreter) error {
	return i.Use(Symbols)
}
