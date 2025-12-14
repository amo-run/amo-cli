package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (e *Environment) GetAllowedCLIPath() string {
	return e.crossPlatform.JoinPath(e.userConfigDir, "allowed_cli.txt")
}

func (e *Environment) EnsureAllowedCLIFile() error {
	filePath := e.GetAllowedCLIPath()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		content := `# Allowed CLI commands for workflows - one per line
# 
# This file controls which CLI commands can be executed within JavaScript workflows.
# It is a security whitelist to prevent unauthorized system access from workflow scripts.
# 
# IMPORTANT: This is NOT for tool installation commands.
# Only add commands that workflows need to execute directly.
#
# Basic system commands (safe for workflows)
echo
#
# Default supported external tools (for workflow processing)
# Media processing
ffmpeg
#
# Image processing
magick
convert
#
# Document conversion and processing
ebook-convert
gs
pandoc
#
# OCR and text extraction
surya_ocr
doc-to-text
#
# LLM and AI tools
llm-caller
#
# Add your custom workflow commands below:
# (Only add commands that workflows need to execute)
#
`
		err := e.crossPlatform.CreateFileWithPermissions(filePath, []byte(content), false)
		if err != nil {
			return fmt.Errorf("failed to create allowed CLI file: %w", err)
		}
	}

	return nil
}

func (e *Environment) LoadAllowedCLICommands() ([]string, error) {
	if err := e.EnsureAllowedCLIFile(); err != nil {
		return nil, err
	}

	filePath := e.GetAllowedCLIPath()
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read allowed CLI file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var commands []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			commands = append(commands, line)
		}
	}

	return commands, nil
}

func (e *Environment) IsCommandAllowed(command string) (bool, error) {
	allowedCommands, err := e.LoadAllowedCLICommands()
	if err != nil {
		return false, err
	}

	if len(allowedCommands) == 0 {
		return false, nil
	}

	for _, allowedCmd := range allowedCommands {
		if allowedCmd == command {
			return true, nil
		}
	}

	return false, nil
}

func (e *Environment) AddAllowedCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	commands, err := e.LoadAllowedCLICommands()
	if err != nil {
		return fmt.Errorf("failed to load current commands: %w", err)
	}

	for _, cmd := range commands {
		if cmd == command {
			return fmt.Errorf("command '%s' is already in the whitelist", command)
		}
	}

	commands = append(commands, command)

	return e.saveAllowedCLICommands(commands)
}

func (e *Environment) RemoveAllowedCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	commands, err := e.LoadAllowedCLICommands()
	if err != nil {
		return fmt.Errorf("failed to load current commands: %w", err)
	}

	var updatedCommands []string
	found := false
	for _, cmd := range commands {
		if cmd != command {
			updatedCommands = append(updatedCommands, cmd)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("command '%s' is not in the whitelist", command)
	}

	return e.saveAllowedCLICommands(updatedCommands)
}

func (e *Environment) saveAllowedCLICommands(commands []string) error {
	filePath := e.GetAllowedCLIPath()

	content := `# Allowed CLI commands for workflows - one per line
# 
# This file controls which CLI commands can be executed within JavaScript workflows.
# It is a security whitelist to prevent unauthorized system access from workflow scripts.
# 
# IMPORTANT: This is NOT for tool installation commands.
# Only add commands that workflows need to execute directly.
#
# Basic system commands (safe for workflows)
# echo
#
# External tools for workflow processing:
#
`

	for _, cmd := range commands {
		if cmd != "" {
			content += cmd + "\n"
		}
	}

	content += "#\n# Add your custom workflow commands above\n"

	dir := filepath.Dir(filePath)
	if err := e.crossPlatform.CreateDirWithPermissions(dir); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return e.crossPlatform.CreateFileWithPermissions(filePath, []byte(content), false)
}
