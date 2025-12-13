package tool

import (
	"context"
	"fmt"

	"amo/pkg/workflow"
)

// WorkflowEngine defines the interface for workflow execution
type WorkflowEngine interface {
	RunWorkflow(workflowName string, params map[string]interface{}) (map[string]interface{}, error)
}

// WorkflowEngineWrapper wraps the pkg/workflow.Engine to implement our interface
type WorkflowEngineWrapper struct {
	engine *workflow.Engine
	ctx    context.Context
}

// NewWorkflowEngineWrapper creates a new wrapper for the workflow engine
func NewWorkflowEngineWrapper(ctx context.Context) *WorkflowEngineWrapper {
	return &WorkflowEngineWrapper{
		engine: workflow.NewEngine(ctx),
		ctx:    ctx,
	}
}

// SetAssetReader sets the asset reader for the wrapped engine
func (w *WorkflowEngineWrapper) SetAssetReader(reader workflow.AssetReader) {
	w.engine.SetAssetReader(reader)
}

// RunWorkflow implements the WorkflowEngine interface
func (w *WorkflowEngineWrapper) RunWorkflow(workflowName string, params map[string]interface{}) (map[string]interface{}, error) {
	vars := make(map[string]string)
	for key, value := range params {
		if str, ok := value.(string); ok {
			vars[key] = str
		} else {
			vars[key] = fmt.Sprintf("%v", value)
		}
	}
	w.engine.SetVars(vars)

	if err := w.engine.RunWorkflow(workflowName); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}

// ToolPathProviderAdapter implements the workflow.ToolPathProvider interface
type ToolPathProviderAdapter struct {
	manager *Manager
}

// NewToolPathProviderAdapter creates a new adapter for the workflow package
func (m *Manager) NewToolPathProviderAdapter() *ToolPathProviderAdapter {
	return &ToolPathProviderAdapter{manager: m}
}

// GetCachedToolPath implements the workflow.ToolPathProvider interface
func (a *ToolPathProviderAdapter) GetCachedToolPath(commandName string) (string, bool) {
	return a.manager.GetCachedToolPath(commandName)
}

