package api

import "time"

const MaxPageSize = 150

type Identity struct {
	Uin           uint   `json:"uin"`
	CompanyID     uint   `json:"company_id"`
	CompanyName   string `json:"company_name"`
	APIKeyID      uint   `json:"api_key_id"`
	APIKeyPurpose string `json:"api_key_purpose"`
}

type CLIAuthStart struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type CLIAuthPoll struct {
	Status        string `json:"status"`
	APIKey        string `json:"api_key,omitempty"`
	APIKeyID      uint   `json:"api_key_id,omitempty"`
	APIKeyPurpose string `json:"api_key_purpose,omitempty"`
	UIN           uint   `json:"uin,omitempty"`
	CompanyID     uint   `json:"company_id,omitempty"`
	CompanyName   string `json:"company_name,omitempty"`
}

type Forest struct {
	ForestID        uint   `json:"forest_id"`
	Name            string `json:"name"`
	KnowledgeStatus string `json:"knowledge_status"`
	Description     string `json:"description"`
	FileCount       int64  `json:"file_count"`
	TotalSize       int64  `json:"total_size"`
}

type ForestPage struct {
	Total  int64    `json:"total"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
	Data   []Forest `json:"data"`
}

type CreateForestResult struct {
	ForestID uint `json:"forest_id"`
}

type ForestFile struct {
	ForestFileID uint      `json:"forest_file_id"`
	ForestID     uint      `json:"forest_id"`
	IsDir        int8      `json:"is_dir"`
	ParentID     uint      `json:"parent_id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Ext          string    `json:"ext"`
	FileStatus   string    `json:"file_status"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

type ForestFilePage struct {
	Total  int64        `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
	Data   []ForestFile `json:"data"`
}

type UploadFileResult struct {
	ForestFileID uint `json:"forest_file_id"`
}

type ChatSession struct {
	SessionID     uint      `json:"session_id"`
	Name          string    `json:"name"`
	ForestFileIDs []uint    `json:"forest_file_id"`
	ForestIDs     []uint    `json:"forest_id"`
	ModelName     string    `json:"model_name"`
	CreatedAt     time.Time `json:"created_at"`
}

type ChatSessionPage struct {
	Total  int64         `json:"total"`
	Offset int           `json:"offset"`
	Limit  int           `json:"limit"`
	Data   []ChatSession `json:"data"`
}

type ChatCompletion struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []ChatChoice   `json:"choices"`
	Usage   map[string]any `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
