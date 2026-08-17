package ext

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
)

// ExtensionAgentTool adapts a ToolDef into fantasy.AgentTool so LLM agents can call it.
type ExtensionAgentTool struct {
	def             ToolDef
	providerOptions fantasy.ProviderOptions
}

// NewExtensionAgentTool creates a new ExtensionAgentTool from a ToolDef.
func NewExtensionAgentTool(def ToolDef) *ExtensionAgentTool {
	return &ExtensionAgentTool{
		def: def,
	}
}

// Info returns the metadata schema for the tool.
func (t *ExtensionAgentTool) Info() fantasy.ToolInfo {
	parameters := make(map[string]any)
	required := make([]string, 0)

	if t.def.Parameters != nil {
		if props, ok := t.def.Parameters["properties"].(map[string]any); ok {
			parameters = props
		} else {
			parameters = t.def.Parameters
		}

		if req, ok := t.def.Parameters["required"].([]any); ok {
			for _, v := range req {
				if s, ok := v.(string); ok {
					required = append(required, s)
				}
			}
		} else if reqStr, ok := t.def.Parameters["required"].([]string); ok {
			required = reqStr
		}
	}

	return fantasy.ToolInfo{
		Name:        t.def.Name,
		Description: t.def.Description,
		Parameters:  parameters,
		Required:    required,
	}
}

// ProviderOptions returns provider-specific options.
func (t *ExtensionAgentTool) ProviderOptions() fantasy.ProviderOptions {
	return t.providerOptions
}

// SetProviderOptions sets provider-specific options.
func (t *ExtensionAgentTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.providerOptions = opts
}

// Run executes the extension tool.
func (t *ExtensionAgentTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if t.def.Execute == nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("tool %s has no execute handler", t.def.Name)), nil
	}

	var args map[string]any
	if call.Input != "" {
		if err := json.Unmarshal([]byte(call.Input), &args); err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to parse arguments: %v", err)), nil
		}
	} else {
		args = make(map[string]any)
	}

	res, err := t.def.Execute(args)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	return fantasy.NewTextResponse(res), nil
}

// ToolDef returns the underlying ToolDef.
func (t *ExtensionAgentTool) ToolDef() ToolDef {
	return t.def
}
