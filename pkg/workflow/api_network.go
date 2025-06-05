package workflow

import (
	"fmt"

	"amo/pkg/network"
)

// registerNetworkAPI registers network functions for JavaScript
func (e *Engine) registerNetworkAPI() {
	if e.network == nil {
		// Network not available, register placeholder functions
		e.vm.Set("http", map[string]interface{}{
			"get":          e.networkNotAvailable,
			"post":         e.networkNotAvailable,
			"getJSON":      e.networkNotAvailable,
			"downloadFile": e.networkNotAvailable,
		})
		return
	}

	// Network available, register actual functions
	e.vm.Set("http", map[string]interface{}{
		"get":          e.httpGet,
		"post":         e.httpPost,
		"getJSON":      e.httpGetJSON,
		"downloadFile": e.httpDownloadFile,
	})
}

// Network operation functions

func (e *Engine) httpGet(url string, headers map[string]interface{}) map[string]interface{} {
	if e.network == nil {
		return map[string]interface{}{
			"error": "Network client not available",
		}
	}

	headerMap := convertHeaders(headers)
	response := e.network.Get(url, headerMap)

	return map[string]interface{}{
		"status_code": response.StatusCode,
		"headers":     response.Headers,
		"body":        response.Body,
		"error":       response.Error,
	}
}

func (e *Engine) httpPost(url string, body string, headers map[string]interface{}) map[string]interface{} {
	if e.network == nil {
		return map[string]interface{}{
			"error": "Network client not available",
		}
	}

	headerMap := convertHeaders(headers)
	response := e.network.Post(url, body, headerMap)

	return map[string]interface{}{
		"status_code": response.StatusCode,
		"headers":     response.Headers,
		"body":        response.Body,
		"error":       response.Error,
	}
}

func (e *Engine) httpGetJSON(url string, headers map[string]interface{}) map[string]interface{} {
	if e.network == nil {
		return map[string]interface{}{
			"error": "Network client not available",
		}
	}

	headerMap := convertHeaders(headers)
	return e.network.GetJSON(url, headerMap)
}

func (e *Engine) httpDownloadFile(url string, outputPath string, options map[string]interface{}) map[string]interface{} {
	if e.network == nil {
		return map[string]interface{}{
			"error": "Network client not available",
		}
	}

	// Parse options
	showProgress := false
	if options != nil {
		if val, ok := options["show_progress"].(bool); ok {
			showProgress = val
		}
	}

	// Progress callback
	var progressCallback func(network.DownloadProgress)
	if showProgress {
		progressCallback = func(progress network.DownloadProgress) {
			fmt.Printf("\rDownloading... %d%% (%s/%s) - %s",
				progress.Percentage,
				formatBytes(progress.Downloaded),
				formatBytes(progress.Total),
				progress.Speed)
		}
	}

	response := e.network.DownloadFile(url, outputPath, progressCallback)

	if showProgress && response.Error == "" {
		fmt.Println() // New line after progress
	}

	return map[string]interface{}{
		"status_code": response.StatusCode,
		"headers":     response.Headers,
		"body":        response.Body,
		"error":       response.Error,
	}
}

func (e *Engine) networkNotAvailable(args ...interface{}) map[string]interface{} {
	return map[string]interface{}{
		"error": "Network functionality not available",
	}
}

// Helper functions

func convertHeaders(headers map[string]interface{}) map[string]string {
	if headers == nil {
		return nil
	}

	result := make(map[string]string)
	for key, value := range headers {
		if strValue, ok := value.(string); ok {
			result[key] = strValue
		} else {
			result[key] = fmt.Sprintf("%v", value)
		}
	}
	return result
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
