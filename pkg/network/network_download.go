package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DownloadProgress struct {
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percentage int    `json:"percentage"`
	Speed      string `json:"speed"`
}

func (nc *NetworkClient) DownloadFile(urlStr, outputPath string, progressCallback func(DownloadProgress)) *HTTPResponse {
	if !nc.isURLAllowed(urlStr) {
		return &HTTPResponse{
			Error: fmt.Sprintf("URL not in allowed hosts whitelist: %s", urlStr),
		}
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	req.Header.Set("User-Agent", "amo-cli/1.0")

	resp, err := nc.client.Do(req)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("HTTP error: %s", resp.Status),
		}
	}

	outputDir := filepath.Dir(outputPath)
	if err := nc.environment.GetCrossPlatformUtils().CreateDirWithPermissions(outputDir); err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("failed to create output directory: %v", err),
		}
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("failed to create output file: %v", err),
		}
	}
	defer outFile.Close()

	contentLength := resp.ContentLength

	var downloaded int64
	buffer := make([]byte, 32*1024)
	startTime := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := outFile.Write(buffer[:n]); writeErr != nil {
				return &HTTPResponse{
					Error: fmt.Sprintf("failed to write to file: %v", writeErr),
				}
			}
			downloaded += int64(n)

			if progressCallback != nil && contentLength > 0 {
				elapsed := time.Since(startTime)
				speed := float64(downloaded) / elapsed.Seconds()
				percentage := int(float64(downloaded) / float64(contentLength) * 100)

				progress := DownloadProgress{
					Downloaded: downloaded,
					Total:      contentLength,
					Percentage: percentage,
					Speed:      formatBytes(int64(speed)) + "/s",
				}
				progressCallback(progress)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return &HTTPResponse{
				Error: fmt.Sprintf("failed to read response: %v", err),
			}
		}
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    nc.extractHeaders(resp.Header),
		Body:       fmt.Sprintf("Downloaded %d bytes to %s", downloaded, outputPath),
	}
}

func (nc *NetworkClient) DownloadFileResume(urlStr, outputPath string, progressCallback func(DownloadProgress)) *HTTPResponse {
	if !nc.isURLAllowed(urlStr) {
		return &HTTPResponse{Error: fmt.Sprintf("URL not in allowed hosts whitelist: %s", urlStr)}
	}

	outputDir := filepath.Dir(outputPath)
	if err := nc.environment.GetCrossPlatformUtils().CreateDirWithPermissions(outputDir); err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to create output directory: %v", err)}
	}

	partPath := outputPath + ".part"
	metaPath := outputPath + ".part.meta"

	var offset int64
	if info, err := os.Stat(partPath); err == nil {
		offset = info.Size()
	}

	buildReq := func(withRange bool) (*http.Request, error) {
		r, e := http.NewRequest("GET", urlStr, nil)
		if e != nil {
			return nil, e
		}
		r.Header.Set("User-Agent", "amo-cli/1.0")
		if withRange && offset > 0 {
			r.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
			if metaBytes, e2 := os.ReadFile(metaPath); e2 == nil && len(metaBytes) > 0 {
				var meta map[string]string
				if json.Unmarshal(metaBytes, &meta) == nil {
					if etag, ok := meta["etag"]; ok && etag != "" {
						r.Header.Set("If-Range", etag)
					} else if lm, ok := meta["last_modified"]; ok && lm != "" {
						r.Header.Set("If-Range", lm)
					}
				}
			}
		}
		return r, nil
	}

	f, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to open part file: %v", err)}
	}

	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to seek part file: %v", err)}
		}
	}

	req, err := buildReq(offset > 0)
	if err != nil {
		_ = f.Close()
		return &HTTPResponse{Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	resp, err := nc.client.Do(req)
	if err != nil {
		_ = f.Close()
		return &HTTPResponse{Error: fmt.Sprintf("request failed: %v", err)}
	}

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset > 0 {
		var total int64 = -1
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if slash := strings.LastIndex(cr, "/"); slash != -1 {
				if t, perr := strconv.ParseInt(strings.TrimSpace(cr[slash+1:]), 10, 64); perr == nil {
					total = t
				}
			}
		}
		resp.Body.Close()
		if total > 0 && offset >= total {
			_ = f.Sync()
			_ = f.Close()
			if _, err := os.Stat(outputPath); err == nil {
				_ = os.Remove(outputPath)
			}
			if err := os.Rename(partPath, outputPath); err != nil {
				return &HTTPResponse{Error: fmt.Sprintf("failed to finalize file: %v", err)}
			}
			_ = os.Remove(metaPath)
			return &HTTPResponse{StatusCode: http.StatusOK, Body: fmt.Sprintf("Downloaded %d bytes to %s", offset, outputPath)}
		}
		if err := f.Truncate(0); err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to reset part file: %v", err)}
		}
		if _, err := f.Seek(0, 0); err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to rewind part file: %v", err)}
		}
		offset = 0
		_ = os.Remove(metaPath)
		req, err = buildReq(false)
		if err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to create request: %v", err)}
		}
		resp, err = nc.client.Do(req)
		if err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("request failed: %v", err)}
		}
	}

	if resp.StatusCode == http.StatusOK && offset > 0 {
		if err := f.Truncate(0); err != nil {
			resp.Body.Close()
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to reset part file: %v", err)}
		}
		if _, err := f.Seek(0, 0); err != nil {
			resp.Body.Close()
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to rewind part file: %v", err)}
		}
		offset = 0
		_ = os.Remove(metaPath)
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		status := resp.Status
		resp.Body.Close()
		_ = f.Close()
		return &HTTPResponse{StatusCode: resp.StatusCode, Error: fmt.Sprintf("HTTP error: %s", status)}
	}

	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	lastModified := strings.TrimSpace(resp.Header.Get("Last-Modified"))
	meta := map[string]string{
		"etag":          etag,
		"last_modified": lastModified,
		"url":           urlStr,
	}
	if metaBytes, err := json.Marshal(meta); err == nil {
		_ = os.WriteFile(metaPath, metaBytes, 0644)
	}

	var total int64 = -1
	if resp.StatusCode == http.StatusPartialContent {
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if slash := strings.LastIndex(cr, "/"); slash != -1 {
				if t, perr := strconv.ParseInt(strings.TrimSpace(cr[slash+1:]), 10, 64); perr == nil {
					total = t
				}
			}
		}
	}
	if total <= 0 && resp.ContentLength > 0 {
		total = offset + resp.ContentLength
	}

	var downloaded int64
	buf := make([]byte, 32*1024)
	startTime := time.Now()
	lastReport := startTime

	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				resp.Body.Close()
				_ = f.Close()
				return &HTTPResponse{Error: fmt.Sprintf("failed to write to part file: %v", werr)}
			}
			downloaded += int64(n)

			if progressCallback != nil {
				now := time.Now()
				if now.Sub(lastReport) >= 200*time.Millisecond || (total > 0 && offset+downloaded == total) {
					elapsed := now.Sub(startTime)
					if elapsed <= 0 {
						elapsed = time.Millisecond
					}
					speed := float64(downloaded) / elapsed.Seconds()
					var percent int
					if total > 0 {
						percent = int(float64(offset+downloaded) / float64(total) * 100)
					}
					progressCallback(DownloadProgress{
						Downloaded: offset + downloaded,
						Total:      total,
						Percentage: percent,
						Speed:      formatBytes(int64(speed)) + "/s",
					})
					lastReport = now
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			resp.Body.Close()
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to read response: %v", rerr)}
		}
	}

	resp.Body.Close()
	_ = f.Sync()
	if err := f.Close(); err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to close part file: %v", err)}
	}
	if _, err := os.Stat(outputPath); err == nil {
		_ = os.Remove(outputPath)
	}
	if err := os.Rename(partPath, outputPath); err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to finalize file: %v", err)}
	}
	_ = os.Remove(metaPath)

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    nc.extractHeaders(resp.Header),
		Body:       fmt.Sprintf("Downloaded %d bytes to %s", offset+downloaded, outputPath),
	}
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
