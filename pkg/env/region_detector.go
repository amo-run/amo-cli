package env

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type RegionScore struct {
	Code  string
	Score float64
	Info  *RegionConfig
}

type RegionDetector struct {
	configs []RegionConfig
}

func NewRegionDetector() *RegionDetector {
	return &RegionDetector{
		configs: GetAllRegionConfigs(),
	}
}

func (d *RegionDetector) DetectRegion() string {
	region, _ := d.DetectRegionWithScore()
	return region
}

func (d *RegionDetector) DetectRegionWithScore() (string, float64) {
	scores := d.calculateAllScores()
	
	if len(scores) == 0 {
		return "global", 0.0
	}
	
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			// Check if region i has a perfect language+country match
			iPerfectMatch := d.hasPerfectLanguageMatch(scores[i].Info)
			jPerfectMatch := d.hasPerfectLanguageMatch(scores[j].Info)
			
			// Prioritize perfect language+country match
			if iPerfectMatch && !jPerfectMatch {
				return true
			}
			if !iPerfectMatch && jPerfectMatch {
				return false
			}
			
			// If both or neither have perfect match, use priority
			return scores[i].Info.Priority < scores[j].Info.Priority
		}
		return scores[i].Score > scores[j].Score
	})
	
	bestMatch := scores[0]
	if bestMatch.Score < 0.3 {
		return "global", 0.0
	}
	
	return bestMatch.Code, bestMatch.Score
}

func (d *RegionDetector) hasPerfectLanguageMatch(config *RegionConfig) bool {
	langVars := []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"}
	
	for _, langVar := range langVars {
		if lang := d.getEnvVar(langVar); lang != "" {
			langLower := strings.ToLower(lang)
			
			// Check for exact language + country matches (e.g., en_CA, en_US)
			for _, prefix := range config.LangPrefixes {
				if strings.HasPrefix(langLower, strings.ToLower(prefix)) {
					// Check if there's a country code after the language
					parts := strings.Split(lang, "_")
					if len(parts) >= 2 {
						// Extract just the country code (remove any encoding info like .UTF-8)
						countryPart := parts[1]
						dotParts := strings.Split(countryPart, ".")
						countryCode := strings.ToUpper(dotParts[0])
						
						for _, configCountry := range config.CountryCodes {
							if countryCode == configCountry {
								return true // Perfect match for language + country
							}
						}
					}
				}
			}
		}
	}
	
	return false
}

func (d *RegionDetector) GetRegionInfo(code string) (*RegionConfig, bool) {
	config := GetRegionConfig(code)
	if config == nil {
		return nil, false
	}
	return config, true
}

func (d *RegionDetector) calculateAllScores() []RegionScore {
	var scores []RegionScore
	
	for _, config := range d.configs {
		score := d.calculateScore(config)
		scores = append(scores, RegionScore{
			Code:  config.Code,
			Score: score,
			Info:  &config,
		})
	}
	
	return scores
}

func (d *RegionDetector) calculateScore(config RegionConfig) float64 {
	var totalWeight float64 = 100
	var weights = map[string]float64{
		"language":  40,
		"timezone":  30,
		"country":   20,
		"misc":      10,
	}
	
	var score float64
	
	langScore := d.calculateLanguageScore(config)
	
	// Bonus for perfect language+country match
	if langScore == 1.0 && d.hasPerfectLanguageMatch(&config) {
		// Add significant bonus to ensure perfect matches win
		score += 0.3
	}
	
	score += (langScore * weights["language"]) / totalWeight
	
	tzScore := d.calculateTimezoneScore(config)
	score += (tzScore * weights["timezone"]) / totalWeight
	
	countryScore := d.calculateCountryScore(config)
	score += (countryScore * weights["country"]) / totalWeight
	
	miscScore := d.calculateMiscScore(config)
	score += (miscScore * weights["misc"]) / totalWeight
	
	return score
}

func (d *RegionDetector) calculateLanguageScore(config RegionConfig) float64 {
	langVars := []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"}
	
	for _, langVar := range langVars {
		if lang := d.getEnvVar(langVar); lang != "" {
			langLower := strings.ToLower(lang)
			
			// First check for exact language + country matches (e.g., en_CA, en_US)
			for _, prefix := range config.LangPrefixes {
				if strings.HasPrefix(langLower, strings.ToLower(prefix)) {
					// Check if there's a country code after the language
					parts := strings.Split(lang, "_")
					if len(parts) >= 2 {
						// Extract just the country code (remove any encoding info like .UTF-8)
						countryPart := parts[1]
						dotParts := strings.Split(countryPart, ".")
						countryCode := strings.ToUpper(dotParts[0])
						
						for _, configCountry := range config.CountryCodes {
							if countryCode == configCountry {
								return 1.0 // Perfect match for language + country
							}
						}
						// Language matches but country doesn't
						return 0.7
					}
					// Only language matches without country
					return 0.5
				}
			}
		}
	}
	
	return 0.0
}

func (d *RegionDetector) calculateTimezoneScore(config RegionConfig) float64 {
	zoneName, offset := time.Now().Zone()
	
	for _, tz := range config.TimeZones {
		if strings.EqualFold(zoneName, tz) {
			return 1.0
		}
	}
	
	for _, configOffset := range config.UTCOffsets {
		if offset == configOffset {
			return 0.8
		}
	}
	
	return 0.0
}

func (d *RegionDetector) calculateCountryScore(config RegionConfig) float64 {
	for _, countryCode := range config.CountryCodes {
		if strings.EqualFold(d.getEnvVar("LOCALE"), countryCode) {
			return 1.0
		}
		
		if locale := d.getEnvVar("LANG"); locale != "" {
			if strings.HasSuffix(strings.ToUpper(locale), "_"+countryCode) {
				return 1.0
			}
		}
	}
	
	return 0.0
}

func (d *RegionDetector) calculateMiscScore(config RegionConfig) float64 {
	var score float64
	var factors int
	
	if d.getEnvVar("AMO_REGION") != "" {
		factors++
		if strings.EqualFold(d.getEnvVar("AMO_REGION"), config.Code) {
			score += 1.0
		}
	}
	
	if factors > 0 {
		return score / float64(factors)
	}
	
	return 0.0
}

func (d *RegionDetector) getEnvVar(name string) string {
	crossPlatform := NewCrossPlatformUtils()
	return crossPlatform.GetEnvironmentVariable(name)
}

func (d *RegionDetector) DebugInfo() map[string]interface{} {
	scores := d.calculateAllScores()
	
	// Use the same sorting logic as DetectRegionWithScore
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			// Check if region i has a perfect language+country match
			iPerfectMatch := d.hasPerfectLanguageMatch(scores[i].Info)
			jPerfectMatch := d.hasPerfectLanguageMatch(scores[j].Info)
			
			// Prioritize perfect language+country match
			if iPerfectMatch && !jPerfectMatch {
				return true
			}
			if !iPerfectMatch && jPerfectMatch {
				return false
			}
			
			// If both or neither have perfect match, use priority
			return scores[i].Info.Priority < scores[j].Info.Priority
		}
		return scores[i].Score > scores[j].Score
	})
	
	debugInfo := map[string]interface{}{
		"system_language":  d.getEnvVar("LANG"),
		"timezone":         func() string { tz, _ := time.Now().Zone(); return tz }(),
		"utc_offset":       func() int { _, offset := time.Now().Zone(); return offset }(),
		"region_override":  d.getEnvVar("AMO_REGION"),
		"scores":           make([]map[string]interface{}, 0, len(scores)),
	}
	
	for _, score := range scores {
		debugInfo["scores"] = append(debugInfo["scores"].([]map[string]interface{}), map[string]interface{}{
			"code":  score.Code,
			"name":  score.Info.Name,
			"score": fmt.Sprintf("%.2f", score.Score),
		})
	}
	
	return debugInfo
}