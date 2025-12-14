package tool

import "fmt"

func (m *Manager) installViaWorkflow(toolName string, installInfo InstallInfo) error {
	workflowName := installInfo.Workflow
	if workflowName == "" {
		return fmt.Errorf("no workflow specified for tool: %s", toolName)
	}

	workflowEngine, err := m.getWorkflowEngine()
	if err != nil {
		return fmt.Errorf("failed to get workflow engine: %w", err)
	}

	params := map[string]interface{}{
		"toolName":   toolName,
		"installDir": m.getInstallDir(),
		"targetFile": installInfo.Target,
		"pattern":    installInfo.Pattern,
	}

	fmt.Printf("🔄 Running installation workflow: %s\n", workflowName)
	result, err := workflowEngine.RunWorkflow(workflowName, params)
	if err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if errorMsg, ok := result["error"].(string); ok {
			return fmt.Errorf("workflow installation failed: %s", errorMsg)
		}
		return fmt.Errorf("workflow installation failed: unknown error")
	}

	fmt.Printf("✅ Workflow completed successfully\n")
	return nil
}

func (m *Manager) getWorkflowEngine() (WorkflowEngine, error) {
	if m.workflowEngine != nil {
		return m.workflowEngine, nil
	}

	return nil, fmt.Errorf("workflow engine not initialized")
}
