package workflow

import (
	"amo/pkg/env"
)

// registerClipboardAPI registers clipboard read/write functions
func (e *Engine) registerClipboardAPI() {
	e.vm.Set("clipboard", map[string]interface{}{
		"read":  e.clipboardRead,
		"write": e.clipboardWrite,
	})
}

// clipboardRead reads plain text from system clipboard
func (e *Engine) clipboardRead() map[string]interface{} {
	cb := env.NewClipboard()
	text, err := cb.ReadText()
	if err != nil {
		return e.createResult(false, nil, err)
	}
	return map[string]interface{}{
		"success": true,
		"text":    text,
	}
}

// clipboardWrite writes plain text to system clipboard
func (e *Engine) clipboardWrite(text string) map[string]interface{} {
	cb := env.NewClipboard()
	if err := cb.WriteText(text); err != nil {
		return e.createResult(false, nil, err)
	}
	return e.createResult(true, nil, nil)
}
