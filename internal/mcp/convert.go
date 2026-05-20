package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/schema"
	"wavelength/internal/llm"
)

// ToLLMTools converts MCP tools to the existing llm.Tool format.
// Each MCP tool is wrapped with an Execute function that routes calls
// back to the MCP manager.
func ToLLMTools(mgr *Manager) []*llm.Tool {
	mcpTools := mgr.Tools()
	if len(mcpTools) == 0 {
		return nil
	}

	result := make([]*llm.Tool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		tool := toLLMTool(mgr, mcpTool)
		if tool != nil {
			result = append(result, tool)
		}
	}

	return result
}

// toLLMTool converts a single MCP tool to an llm.Tool.
func toLLMTool(mgr *Manager, mcpTool *Tool) *llm.Tool {
	params := buildParamsOneOf(mcpTool.InputSchema)
	if params == nil {
		log.Printf("[MCP] Skipping tool %q: could not build parameter schema", mcpTool.Name)
		return nil
	}

	return &llm.Tool{
		Info: &schema.ToolInfo{
			Name:        mcpTool.Name,
			Desc:        mcpTool.Description,
			ParamsOneOf: params,
		},
		Execute: func(ctx context.Context, argsJSON string) (string, error) {
			return executeMCPTool(ctx, mgr, mcpTool.Name, argsJSON)
		},
	}
}

// buildParamsOneOf converts a JSON Schema map to eino's ParamsOneOf.
func buildParamsOneOf(inputSchema map[string]interface{}) *schema.ParamsOneOf {
	if len(inputSchema) == 0 {
		return schema.NewParamsOneOfByParams(nil)
	}

	props, ok := inputSchema["properties"].(map[string]interface{})
	if !ok {
		props = nil
	}

	required, _ := inputSchema["required"].([]interface{})
	requiredSet := make(map[string]bool)
	for _, r := range required {
		if name, ok := r.(string); ok {
			requiredSet[name] = true
		}
	}

	params := make(map[string]*schema.ParameterInfo)
	for name, prop := range props {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		paramInfo := &schema.ParameterInfo{
			Type:     jsonSchemaTypeToEinoType(propMap["type"]),
			Desc:     toString(propMap["description"]),
			Required: requiredSet[name],
		}

		// Handle enum values
		if enum, ok := propMap["enum"].([]interface{}); ok {
			paramInfo.Enum = make([]string, 0, len(enum))
			for _, v := range enum {
				if s, ok := v.(string); ok {
					paramInfo.Enum = append(paramInfo.Enum, s)
				}
			}
		}

		params[name] = paramInfo
	}

	return schema.NewParamsOneOfByParams(params)
}

// jsonSchemaTypeToEinoType maps JSON Schema type strings to eino schema DataType.
func jsonSchemaTypeToEinoType(typeVal interface{}) schema.DataType {
	switch t := typeVal.(type) {
	case string:
		switch t {
		case "string":
			return schema.String
		case "number":
			return schema.Number
		case "integer":
			return schema.Integer
		case "boolean":
			return schema.Boolean
		case "array":
			return schema.Array
		case "object":
			return schema.Object
		default:
			return schema.String
		}
	default:
		return schema.String
	}
}

// executeMCPTool parses the JSON args and calls the MCP manager.
func executeMCPTool(ctx context.Context, mgr *Manager, toolName string, argsJSON string) (string, error) {
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("failed to parse tool arguments: %w", err)
		}
	}

	log.Printf("[MCP-TOOL] Executing %q", toolName)
	result, err := mgr.CallTool(ctx, toolName, args)
	if err != nil {
		log.Printf("[MCP-TOOL] %q failed: %v", toolName, err)
		return result, err // Return partial result if available (for LLM to see error)
	}

	log.Printf("[MCP-TOOL] %q completed (%d bytes result)", toolName, len(result))
	return result, nil
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
