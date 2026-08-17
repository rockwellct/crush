package ext

import (
	"fmt"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// LuaProvider manages Lua runtime scripts using gopher-lua.
type LuaProvider struct {
	mu sync.Mutex
}

// NewLuaProvider creates a new LuaProvider.
func NewLuaProvider() *LuaProvider {
	return &LuaProvider{}
}

// LoadFile executes a Lua script file and binds the crush host API.
func (p *LuaProvider) LoadFile(path string, api API) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	L := lua.NewState()
	// Do not close immediately so background callbacks can be executed,
	// but manage its lifecycle if needed.

	// Register global crush table
	crushTable := L.NewTable()

	// Constants
	L.SetField(crushTable, "ALLOW", lua.LString(string(AllowOnce)))
	L.SetField(crushTable, "ALWAYS_ALLOW", lua.LString(string(AlwaysAllow)))
	L.SetField(crushTable, "DENY", lua.LString(string(Deny)))

	// crush.on(event, handler)
	L.SetField(crushTable, "on", L.NewFunction(func(ls *lua.LState) int {
		eventName := ls.CheckString(1)
		fn := ls.CheckFunction(2)

		api.On(eventName, func(evt Event) (any, error) {
			p.mu.Lock()
			defer p.mu.Unlock()

			evtTable := ls.NewTable()
			ls.SetField(evtTable, "name", lua.LString(evt.Name))
			ls.SetField(evtTable, "session_id", lua.LString(evt.SessionID))
			ls.SetField(evtTable, "prompt", lua.LString(evt.Prompt))
			ls.SetField(evtTable, "tool", lua.LString(evt.Tool))
			ls.SetField(evtTable, "command", lua.LString(evt.Command))
			ls.SetField(evtTable, "result", lua.LString(evt.Result))
			ls.SetField(evtTable, "message", lua.LString(evt.Message))

			if evt.Args != nil {
				argsTable := goMapToLuaTable(ls, evt.Args)
				ls.SetField(evtTable, "args", argsTable)
			}

			if evt.Data != nil {
				dataTable := goMapToLuaTable(ls, evt.Data)
				ls.SetField(evtTable, "data", dataTable)
			}

			err := ls.CallByParam(lua.P{
				Fn:      fn,
				NRet:    1,
				Protect: true,
			}, evtTable)
			if err != nil {
				return nil, err
			}

			ret := ls.Get(-1)
			ls.Pop(1)

			if ret == lua.LNil {
				return nil, nil
			}

			switch ret.Type() {
			case lua.LTString:
				strVal := ret.String()
				if strVal == string(AllowOnce) || strVal == string(AlwaysAllow) || strVal == string(Deny) {
					return ToolDecision(strVal), nil
				}
				return strVal, nil
			case lua.LTBool:
				if !lua.LVAsBool(ret) {
					return Deny, nil
				}
				return AllowOnce, nil
			default:
				return ret.String(), nil
			}
		})

		return 0
	}))

	// crush.register_tool(tbl)
	L.SetField(crushTable, "register_tool", L.NewFunction(func(ls *lua.LState) int {
		tbl := ls.CheckTable(1)

		name := ls.GetField(tbl, "name").String()
		desc := ls.GetField(tbl, "description").String()
		paramsVal := ls.GetField(tbl, "parameters")
		execVal := ls.GetField(tbl, "execute")

		if name == "" {
			ls.ArgError(1, "tool name is required")
			return 0
		}

		var params map[string]any
		if paramsTbl, ok := paramsVal.(*lua.LTable); ok {
			params = luaTableToGoMap(paramsTbl)
		}

		var execFn func(args map[string]any) (string, error)
		if execLuaFn, ok := execVal.(*lua.LFunction); ok {
			execFn = func(args map[string]any) (string, error) {
				p.mu.Lock()
				defer p.mu.Unlock()

				argsTbl := goMapToLuaTable(ls, args)
				err := ls.CallByParam(lua.P{
					Fn:      execLuaFn,
					NRet:    1,
					Protect: true,
				}, argsTbl)
				if err != nil {
					return "", err
				}

				ret := ls.Get(-1)
				ls.Pop(1)
				return ret.String(), nil
			}
		}

		err := api.RegisterTool(ToolDef{
			Name:        name,
			Description: desc,
			Parameters:  params,
			Execute:     execFn,
		})
		if err != nil {
			ls.RaiseError("failed to register tool %s: %v", name, err)
		}

		return 0
	}))

	// crush.register_command(name, desc, fn)
	L.SetField(crushTable, "register_command", L.NewFunction(func(ls *lua.LState) int {
		name := ls.CheckString(1)
		desc := ls.CheckString(2)
		fn := ls.CheckFunction(3)

		api.RegisterCommand(name, desc, func(args []string) error {
			p.mu.Lock()
			defer p.mu.Unlock()

			argsTbl := ls.NewTable()
			for _, arg := range args {
				argsTbl.Append(lua.LString(arg))
			}

			err := ls.CallByParam(lua.P{
				Fn:      fn,
				NRet:    0,
				Protect: true,
			}, argsTbl)

			return err
		})

		return 0
	}))

	// crush.register_keybinding(key, desc, fn)
	L.SetField(crushTable, "register_keybinding", L.NewFunction(func(ls *lua.LState) int {
		key := ls.CheckString(1)
		desc := ls.CheckString(2)
		fn := ls.CheckFunction(3)

		api.RegisterKeybinding(key, desc, func() error {
			p.mu.Lock()
			defer p.mu.Unlock()

			err := ls.CallByParam(lua.P{
				Fn:      fn,
				NRet:    0,
				Protect: true,
			})

			return err
		})

		return 0
	}))

	// crush.notify(msg, level)
	L.SetField(crushTable, "notify", L.NewFunction(func(ls *lua.LState) int {
		msg := ls.CheckString(1)
		level := NotificationLevel(ls.OptString(2, "info"))
		api.Notify(msg, level)
		return 0
	}))

	// crush.get_cwd()
	L.SetField(crushTable, "get_cwd", L.NewFunction(func(ls *lua.LState) int {
		ls.Push(lua.LString(api.GetCwd()))
		return 1
	}))

	// crush.get_session_name()
	L.SetField(crushTable, "get_session_name", L.NewFunction(func(ls *lua.LState) int {
		ls.Push(lua.LString(api.GetSessionName()))
		return 1
	}))

	// crush.get_model()
	L.SetField(crushTable, "get_model", L.NewFunction(func(ls *lua.LState) int {
		ls.Push(lua.LString(api.GetModel()))
		return 1
	}))

	L.SetGlobal("crush", crushTable)

	if err := L.DoFile(path); err != nil {
		return fmt.Errorf("lua execution failed for %s: %w", path, err)
	}

	return nil
}

func goMapToLuaTable(L *lua.LState, m map[string]any) *lua.LTable {
	tbl := L.NewTable()
	for k, v := range m {
		tbl.RawSetString(k, goToLuaValue(L, v))
	}
	return tbl
}

func goToLuaValue(L *lua.LState, val any) lua.LValue {
	if val == nil {
		return lua.LNil
	}
	switch v := val.(type) {
	case string:
		return lua.LString(v)
	case int:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case bool:
		return lua.LBool(v)
	case map[string]any:
		return goMapToLuaTable(L, v)
	case []any:
		tbl := L.NewTable()
		for _, item := range v {
			tbl.Append(goToLuaValue(L, item))
		}
		return tbl
	case []string:
		tbl := L.NewTable()
		for _, item := range v {
			tbl.Append(lua.LString(item))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

func luaTableToGoMap(tbl *lua.LTable) map[string]any {
	result := make(map[string]any)
	tbl.ForEach(func(k, v lua.LValue) {
		keyStr := k.String()
		result[keyStr] = luaValueToGo(v)
	})
	return result
}

func luaValueToGo(val lua.LValue) any {
	switch val.Type() {
	case lua.LTNil:
		return nil
	case lua.LTBool:
		return lua.LVAsBool(val)
	case lua.LTNumber:
		return float64(val.(lua.LNumber))
	case lua.LTString:
		return val.String()
	case lua.LTTable:
		tbl := val.(*lua.LTable)
		// Check if it's an array or map
		maxN := tbl.MaxN()
		if maxN > 0 {
			arr := make([]any, 0, maxN)
			for i := 1; i <= maxN; i++ {
				arr = append(arr, luaValueToGo(tbl.RawGetInt(i)))
			}
			return arr
		}
		return luaTableToGoMap(tbl)
	default:
		return val.String()
	}
}
