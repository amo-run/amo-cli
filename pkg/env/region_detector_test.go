package env

import (
	"os"
	"testing"
	"time"
)

func TestRegionDetector_DetectRegion(t *testing.T) {
	detector := NewRegionDetector()
	
	region := detector.DetectRegion()
	if region == "" {
		t.Error("Expected region to be non-empty")
	}
	
	t.Logf("Detected region: %s", region)
}

func TestRegionDetector_DetectRegionWithScore(t *testing.T) {
	detector := NewRegionDetector()
	
	region, score := detector.DetectRegionWithScore()
	if region == "" {
		t.Error("Expected region to be non-empty")
	}
	
	if score < 0 || score > 1 {
		t.Errorf("Expected score to be between 0 and 1, got %f", score)
	}
	
	t.Logf("Detected region: %s with score: %f", region, score)
}

func TestRegionDetector_GetRegionInfo(t *testing.T) {
	detector := NewRegionDetector()
	
	info, exists := detector.GetRegionInfo("cn")
	if !exists {
		t.Error("Expected China region config to exist")
	}
	
	if info.Code != "cn" {
		t.Errorf("Expected region code to be 'cn', got '%s'", info.Code)
	}
	
	if info.Name != "China" {
		t.Errorf("Expected region name to be 'China', got '%s'", info.Name)
	}
	
	_, exists = detector.GetRegionInfo("invalid")
	if exists {
		t.Error("Expected invalid region config to not exist")
	}
}

func TestRegionDetector_WithEnvOverride(t *testing.T) {
	originalValue := os.Getenv("AMO_REGION")
	defer func() {
		if originalValue != "" {
			os.Setenv("AMO_REGION", originalValue)
		} else {
			os.Unsetenv("AMO_REGION")
		}
	}()
	
	testCases := []struct {
		envValue string
		expected string
	}{
		{"cn", "cn"},
		{"us", "us"},
		{"eu", "eu"},
		{"CN", "cn"},
		{"US", "us"},
		{"EU", "eu"},
	}
	
	for _, tc := range testCases {
		os.Setenv("AMO_REGION", tc.envValue)
		
		environment, _ := NewEnvironment()
		region := environment.DetectRegion()
		
		if region != tc.expected {
			t.Errorf("Expected region to be '%s' when AMO_REGION is '%s', got '%s'", 
				tc.expected, tc.envValue, region)
		}
	}
}

func TestRegionDetector_ChineseRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "zh_CN.UTF-8")
	os.Setenv("TZ", "Asia/Shanghai")
	
	var err error
	time.Local, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("Cannot load Asia/Shanghai timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "cn" {
		t.Errorf("Expected region to be 'cn' for Chinese system, got '%s'", region)
	}
}

func TestRegionDetector_USRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "en_US.UTF-8")
	os.Setenv("TZ", "America/New_York")
	
	var err error
	time.Local, err = time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("Cannot load America/New_York timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "us" {
		t.Errorf("Expected region to be 'us' for US system, got '%s'", region)
	}
}

func TestRegionDetector_JapanRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "ja_JP.UTF-8")
	os.Setenv("TZ", "Asia/Tokyo")
	
	var err error
	time.Local, err = time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("Cannot load Asia/Tokyo timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "jp" {
		t.Errorf("Expected region to be 'jp' for Japanese system, got '%s'", region)
	}
}

func TestRegionDetector_DebugInfo(t *testing.T) {
	detector := NewRegionDetector()
	
	debugInfo := detector.DebugInfo()
	if debugInfo == nil {
		t.Error("Expected debug info to be non-nil")
	}
	
	if _, exists := debugInfo["system_language"]; !exists {
		t.Error("Expected debug info to contain system_language")
	}
	
	if _, exists := debugInfo["timezone"]; !exists {
		t.Error("Expected debug info to contain timezone")
	}
	
	if _, exists := debugInfo["scores"]; !exists {
		t.Error("Expected debug info to contain scores")
	}
}

func TestGetRegionConfig(t *testing.T) {
	config := GetRegionConfig("cn")
	if config == nil {
		t.Error("Expected China region config to exist")
	}
	
	if config.Code != "cn" {
		t.Errorf("Expected region code to be 'cn', got '%s'", config.Code)
	}
	
	config = GetRegionConfig("invalid")
	if config != nil {
		t.Error("Expected invalid region config to be nil")
	}
}

func TestGetAllRegionConfigs(t *testing.T) {
	configs := GetAllRegionConfigs()
	if len(configs) == 0 {
		t.Error("Expected at least one region config")
	}
	
	expectedCodes := []string{"cn", "us", "eu", "jp", "kr", "in", "sg", "au", "br", "ru", "ca", "mx", "za", "eg", "ar", "sa", "th", "id", "my", "ph", "vn", "tr", "il", "uae", "ng", "ke", "pk", "bd", "lk", "np"}
	codeMap := make(map[string]bool)
	for _, config := range configs {
		codeMap[config.Code] = true
	}
	
	for _, code := range expectedCodes {
		if !codeMap[code] {
			t.Errorf("Expected region config for '%s' to exist", code)
		}
	}
}

func TestRegionDetector_BrazilRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "pt_BR.UTF-8")
	os.Setenv("TZ", "America/Sao_Paulo")
	
	var err error
	time.Local, err = time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skipf("Cannot load America/Sao_Paulo timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "br" {
		t.Errorf("Expected region to be 'br' for Brazilian system, got '%s'", region)
	}
}

func TestRegionDetector_RussiaRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "ru_RU.UTF-8")
	os.Setenv("TZ", "Europe/Moscow")
	
	var err error
	time.Local, err = time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Skipf("Cannot load Europe/Moscow timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "ru" {
		t.Errorf("Expected region to be 'ru' for Russian system, got '%s'", region)
	}
}

func TestRegionDetector_CanadaRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "en_CA.UTF-8")
	os.Setenv("TZ", "America/Toronto")
	
	var err error
	time.Local, err = time.LoadLocation("America/Toronto")
	if err != nil {
		t.Skipf("Cannot load America/Toronto timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "ca" {
		t.Errorf("Expected region to be 'ca' for Canadian system, got '%s'", region)
	}
}

func TestRegionDetector_MexicoRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "es_MX.UTF-8")
	os.Setenv("TZ", "America/Mexico_City")
	
	var err error
	time.Local, err = time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Skipf("Cannot load America/Mexico_City timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "mx" {
		t.Errorf("Expected region to be 'mx' for Mexican system, got '%s'", region)
	}
}

func TestRegionDetector_ThailandRegion(t *testing.T) {
	originalLang := os.Getenv("LANG")
	originalTZ := os.Getenv("TZ")
	defer func() {
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		time.Local = time.UTC
	}()
	
	os.Setenv("LANG", "th_TH.UTF-8")
	os.Setenv("TZ", "Asia/Bangkok")
	
	var err error
	time.Local, err = time.LoadLocation("Asia/Bangkok")
	if err != nil {
		t.Skipf("Cannot load Asia/Bangkok timezone: %v", err)
		return
	}
	
	environment, _ := NewEnvironment()
	region := environment.DetectRegion()
	
	if region != "th" {
		t.Errorf("Expected region to be 'th' for Thai system, got '%s'", region)
	}
}