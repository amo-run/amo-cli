package tool

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func (m *Manager) installViaWorkflow(toolName string, installInfo InstallInfo) error {
	workflowName := installInfo.Workflow
	if workflowName == "" {
		return fmt.Errorf("no workflow specified for tool: %s", toolName)
	}

	workflowEngine, err := m.getWorkflowEngine()
	if err != nil {
		return fmt.Errorf("failed to get workflow engine: %w", err)
	}

	downloadURL := installInfo.URL
	if m.isChinaRegion() && installInfo.MirrorURL != "" {
		downloadURL = installInfo.MirrorURL
		fmt.Printf("Using mirror URL for China region: %s\n", downloadURL)
	}

	params := map[string]interface{}{
		"toolName":      toolName,
		"installDir":    m.getInstallDir(),
		"targetFile":    installInfo.Target,
		"portableUrl":   installInfo.PortableURL,
		"mirrorUrl":     installInfo.MirrorURL,
		"url":           downloadURL,
		"pattern":       installInfo.Pattern,
		"isChinaRegion": m.isChinaRegion(),
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

func (m *Manager) isChinaRegion() bool {
	if lang := os.Getenv("LANG"); lang != "" {
		if strings.Contains(lang, "zh_CN") || strings.Contains(lang, "zh-CN") {
			return true
		}
	}

	if lc := os.Getenv("LC_ALL"); lc != "" {
		if strings.Contains(lc, "zh_CN") || strings.Contains(lc, "zh-CN") {
			return true
		}
	}

	if tz := os.Getenv("TZ"); tz != "" {
		if strings.Contains(tz, "Shanghai") || strings.Contains(tz, "Beijing") || strings.Contains(tz, "Chongqing") {
			return true
		}
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-Command", "(Get-WinSystemLocale).Name")
		if output, err := cmd.Output(); err == nil {
			locale := strings.TrimSpace(string(output))
			if strings.HasPrefix(locale, "zh-CN") {
				return true
			}
		}
	}

	if os.Getenv("CHINA_MIRROR") == "true" || os.Getenv("AMO_USE_CHINA_MIRROR") == "true" {
		return true
	}

	return false
}
