package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	MaxMatches           = 2000
	MaxMatchContentBytes = 4000
	MaxResponseBytes     = 1 << 20
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type AnalysisRequest struct {
	TaskID     string       `json:"task_id"`
	UploadID   string       `json:"upload_id"`
	File       AnalysisFile `json:"file"`
	TotalLines int          `json:"total_lines"`
	Matches    []Match      `json:"matches"`
}

type AnalysisFile struct {
	ID           int64  `json:"id"`
	RelativePath string `json:"relative_path"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	LineCount    int    `json:"line_count"`
}

type Match struct {
	Level       string `json:"level"`
	MatchedText string `json:"matched_text"`
	LineNumber  int    `json:"line_number"`
	Content     string `json:"content"`
	FilePath    string `json:"file_path"`
}

type AnalysisResponse struct {
	Summary  string    `json:"summary"`
	Findings []Finding `json:"findings"`
}

type Finding struct {
	Category   string  `json:"category"`
	Severity   string  `json:"severity"`
	RootCause  string  `json:"root_cause"`
	Suggestion string  `json:"suggestion"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

var validCategories = map[string]struct{}{
	"system": {}, "camera": {}, "gps": {}, "storage": {},
	"sensor": {}, "network": {}, "recording": {}, "unknown": {},
}

var validSeverities = map[string]struct{}{
	"warning": {}, "error": {}, "critical": {},
}

func ValidateRequest(req AnalysisRequest) error {
	if !uuidPattern.MatchString(req.TaskID) {
		return errors.New("task_id must be a UUID")
	}
	if !uuidPattern.MatchString(req.UploadID) {
		return errors.New("upload_id must be a UUID")
	}
	if req.File.ID <= 0 {
		return errors.New("file.id must be positive")
	}
	if strings.TrimSpace(req.File.RelativePath) == "" {
		return errors.New("file.relative_path is required")
	}
	if req.File.SizeBytes < 0 || req.File.LineCount < 0 || req.TotalLines < 0 {
		return errors.New("size and line counts cannot be negative")
	}
	if len(req.File.SHA256) != sha256.Size*2 {
		return errors.New("file.sha256 must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(req.File.SHA256); err != nil {
		return errors.New("file.sha256 must contain 64 hexadecimal characters")
	}
	if len(req.Matches) > MaxMatches {
		return fmt.Errorf("matches cannot exceed %d entries", MaxMatches)
	}
	for i, match := range req.Matches {
		if match.LineNumber <= 0 {
			return fmt.Errorf("matches[%d].line_number must be positive", i)
		}
		if len(match.Content) > MaxMatchContentBytes {
			return fmt.Errorf("matches[%d].content cannot exceed %d bytes", i, MaxMatchContentBytes)
		}
	}
	return nil
}

func ValidateResponse(response AnalysisResponse) error {
	if strings.TrimSpace(response.Summary) == "" {
		return errors.New("analysis summary is required")
	}
	if response.Findings == nil {
		return errors.New("analysis findings must be an array")
	}
	for i, finding := range response.Findings {
		if _, ok := validCategories[finding.Category]; !ok {
			return fmt.Errorf("findings[%d].category is invalid", i)
		}
		if _, ok := validSeverities[finding.Severity]; !ok {
			return fmt.Errorf("findings[%d].severity is invalid", i)
		}
		if strings.TrimSpace(finding.RootCause) == "" || strings.TrimSpace(finding.Suggestion) == "" || strings.TrimSpace(finding.Evidence) == "" {
			return fmt.Errorf("findings[%d] is incomplete", i)
		}
		if math.IsNaN(finding.Confidence) || math.IsInf(finding.Confidence, 0) || finding.Confidence < 0 || finding.Confidence > 1 {
			return fmt.Errorf("findings[%d].confidence must be between 0 and 1", i)
		}
	}
	return nil
}

func AnalysisKey(req AnalysisRequest) string {
	value := req.TaskID + ":" + strconv.FormatInt(req.File.ID, 10) + ":" + strings.ToLower(req.File.SHA256)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func decodeStrictJSON(data []byte, value any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("JSON must contain exactly one value")
	}
	return nil
}
