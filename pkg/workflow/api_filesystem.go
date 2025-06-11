package workflow

import "amo/pkg/filesystem"

// registerFileSystemAPI registers all file system related functions
func (e *Engine) registerFileSystemAPI() {
	// File/Directory checks
	e.vm.Set("fs", map[string]interface{}{
		// File/Directory checks
		"exists": e.exists,
		"isFile": e.isFile,
		"isDir":  e.isDir,
		"info":   e.getFileInfo,
		"stat":   e.getFileInfo, // alias

		// Directory operations
		"readdir": e.listDir,
		"list":    e.listDir, // alias
		"mkdir":   e.makeDir,

		// File operations
		"read":       e.readFile,
		"readFile":   e.readFile, // alias
		"write":      e.writeFile,
		"writeFile":  e.writeFile, // alias
		"append":     e.appendFile,
		"appendFile": e.appendFile, // alias
		"copy":       e.copyFile,
		"move":       e.moveFile,
		"rename":     e.moveFile, // alias
		"remove":     e.deleteFile,
		"delete":     e.deleteFile, // alias
		"rm":         e.deleteFile, // alias

		// Path operations
		"join":     e.joinPath,
		"split":    e.splitPath,
		"abs":      e.getAbsolutePath,
		"absolute": e.getAbsolutePath, // alias
		"rel":      e.getRelativePath,
		"relative": e.getRelativePath, // alias
		"ext":      e.getExtension,
		"extname":  e.getExtension, // alias
		"filename": e.getFileName,
		"basename": e.getBaseName,
		"dirname":  e.getDirName,

		// Utilities
		"size":   e.getFileSize,
		"find":   e.findFiles,
		"search": e.findFiles, // alias
		"cwd":    e.getWorkingDir,
		"getcwd": e.getWorkingDir, // alias
		"chdir":  e.changeDir,
		"cd":     e.changeDir, // alias
	})
}

// Helper function to create standard result map
func (e *Engine) createResult(success bool, data interface{}, err error) map[string]interface{} {
	result := map[string]interface{}{
		"success": success,
	}
	if data != nil {
		result["data"] = data
	}
	if err != nil {
		result["error"] = err.Error()
	}
	return result
}

// File/Directory checks
func (e *Engine) isFile(path string) bool {
	return e.filesystem.IsFile(path)
}

func (e *Engine) isDir(path string) bool {
	return e.filesystem.IsDir(path)
}

func (e *Engine) exists(path string) bool {
	return e.filesystem.Exists(path)
}

func (e *Engine) getFileInfo(path string) map[string]interface{} {
	info, err := e.filesystem.GetFileInfo(path)
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return e.createResult(true, fileInfoToMap(*info), nil)
}

// Directory operations
func (e *Engine) listDir(dirPath string) map[string]interface{} {
	files, err := e.filesystem.List(dirPath)
	if err != nil {
		return e.createResult(false, nil, err)
	}

	// Convert []filesystem.FileInfo to []map[string]interface{} for goja
	interfaceFiles := make([]interface{}, len(files))
	for i, f := range files {
		interfaceFiles[i] = fileInfoToMap(f)
	}

	return map[string]interface{}{
		"success": true,
		"files":   interfaceFiles,
	}
}

func (e *Engine) makeDir(dirPath string) map[string]interface{} {
	err := e.filesystem.MakeDir(dirPath)
	return e.createResult(err == nil, nil, err)
}

// File operations
func (e *Engine) copyFile(src, dst string) map[string]interface{} {
	err := e.filesystem.Copy(src, dst)
	return e.createResult(err == nil, nil, err)
}

func (e *Engine) moveFile(src, dst string) map[string]interface{} {
	err := e.filesystem.Move(src, dst)
	return e.createResult(err == nil, nil, err)
}

func (e *Engine) deleteFile(path string) map[string]interface{} {
	err := e.filesystem.Delete(path)
	return e.createResult(err == nil, nil, err)
}

func (e *Engine) readFile(path string) map[string]interface{} {
	content, err := e.filesystem.ReadFile(path)
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return map[string]interface{}{
		"success": true,
		"content": content,
	}
}

func (e *Engine) writeFile(path, content string) map[string]interface{} {
	err := e.filesystem.WriteFile(path, content)
	return e.createResult(err == nil, nil, err)
}

func (e *Engine) appendFile(path, content string) map[string]interface{} {
	err := e.filesystem.AppendFile(path, content)
	return e.createResult(err == nil, nil, err)
}

// Utilities
func (e *Engine) getFileSize(path string) map[string]interface{} {
	size, err := e.filesystem.GetSize(path)
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return map[string]interface{}{
		"success": true,
		"size":    size,
	}
}

func (e *Engine) findFiles(rootPath, pattern string) map[string]interface{} {
	files, err := e.filesystem.Find(rootPath, pattern)
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return map[string]interface{}{
		"success": true,
		"files":   files,
	}
}

// Working directory operations
func (e *Engine) getWorkingDir() map[string]interface{} {
	dir, err := e.filesystem.GetWorkingDir()
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return map[string]interface{}{
		"success": true,
		"path":    dir,
	}
}

func (e *Engine) changeDir(path string) map[string]interface{} {
	err := e.filesystem.ChangeDir(path)
	return e.createResult(err == nil, nil, err)
}

// Path operations
func (e *Engine) getAbsolutePath(path string) map[string]interface{} {
	absPath, err := e.filesystem.GetAbsolutePath(path)
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return map[string]interface{}{
		"success": true,
		"path":    absPath,
	}
}

func (e *Engine) getRelativePath(base, target string) map[string]interface{} {
	relPath, err := e.filesystem.GetRelativePath(base, target)
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return map[string]interface{}{
		"success": true,
		"path":    relPath,
	}
}

func (e *Engine) joinPath(elements []string) string {
	return e.filesystem.JoinPath(elements...)
}

func (e *Engine) splitPath(path string) map[string]interface{} {
	dir, file := e.filesystem.SplitPath(path)
	return map[string]interface{}{
		"dir":  dir,
		"file": file,
	}
}

func (e *Engine) getExtension(path string) string {
	return e.filesystem.GetExtension(path)
}

func (e *Engine) getFileName(path string) string {
	return e.filesystem.GetFileName(path)
}

func (e *Engine) getBaseName(path string) string {
	return e.filesystem.GetBaseName(path)
}

func (e *Engine) getDirName(path string) string {
	return e.filesystem.GetDirName(path)
}

func fileInfoToMap(info filesystem.FileInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":     info.Name,
		"path":     info.Path,
		"size":     info.Size,
		"is_dir":   info.IsDir,
		"mod_time": info.ModTime,
		"mode":     info.Mode,
	}
}
