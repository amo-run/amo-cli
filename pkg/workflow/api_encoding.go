package workflow

import (
	"encoding/base64"
)

// registerEncodingAPI registers encoding/decoding related APIs
func (e *Engine) registerEncodingAPI() {
	e.vm.Set("encoding", map[string]interface{}{
		// Base64 functions
		"base64Encode": e.base64Encode,
		"base64Decode": e.base64Decode,

		// 未来可以添加更多编码功能，如：
		// "urlEncode":   e.urlEncode,
		// "urlDecode":   e.urlDecode,
		// "hexEncode":   e.hexEncode,
		// "hexDecode":   e.hexDecode,
		// "md5":         e.md5Encode,
	})
}

// base64Encode encodes a string to base64
func (e *Engine) base64Encode(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}

// base64Decode decodes a base64 string to a regular string
func (e *Engine) base64Decode(input string) map[string]interface{} {
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
		"text":    string(decoded),
	}
}
