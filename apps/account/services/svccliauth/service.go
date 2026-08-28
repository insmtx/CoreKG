package svccliauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/insmtx/corekg/apps/account/models/apikey"
	"github.com/redis/go-redis/v9"
	"github.com/ygpkg/yg-go/dbtools/redispool"
)

const (
	sessionTTL   = 10 * time.Minute
	defaultPoll  = 5
	userCodeSize = 8
)

var userCodeAlphabet = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

type Session struct {
	DeviceCode    string `json:"device_code"`
	UserCode      string `json:"user_code"`
	ClientName    string `json:"client_name,omitempty"`
	CLIVersion    string `json:"cli_version,omitempty"`
	Status        string `json:"status"`
	UIN           uint   `json:"uin,omitempty"`
	CompanyID     uint   `json:"company_id,omitempty"`
	CompanyName   string `json:"company_name,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	APIKeyID      uint   `json:"api_key_id,omitempty"`
	APIKeyPurpose string `json:"api_key_purpose,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix"`
	ExpiresAtUnix int64  `json:"expires_at_unix"`
}

type StartResult struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       int
	Interval        int
}

type PollResult struct {
	Status        string
	APIKey        string
	APIKeyID      uint
	APIKeyPurpose string
	UIN           uint
	CompanyID     uint
	CompanyName   string
}

func Start(ctx context.Context, clientName, cliVersion, verificationURI string) (*StartResult, error) {
	deviceCode, err := randomToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate device code: %w", err)
	}
	userCode, err := randomUserCode(userCodeSize)
	if err != nil {
		return nil, fmt.Errorf("generate user code: %w", err)
	}
	now := time.Now().UTC()
	session := Session{
		DeviceCode:    deviceCode,
		UserCode:      userCode,
		ClientName:    strings.TrimSpace(clientName),
		CLIVersion:    strings.TrimSpace(cliVersion),
		Status:        "pending",
		CreatedAtUnix: now.Unix(),
		ExpiresAtUnix: now.Add(sessionTTL).Unix(),
	}
	rdb := redispool.Redis()
	deviceKey := sessionKey(deviceCode)
	userKey := userCodeKey(userCode)
	if err := rdb.Set(ctx, deviceKey, marshalSession(session), sessionTTL).Err(); err != nil {
		return nil, fmt.Errorf("save device authorization: %w", err)
	}
	if err := rdb.Set(ctx, userKey, deviceCode, sessionTTL).Err(); err != nil {
		_ = rdb.Del(ctx, deviceKey).Err()
		return nil, fmt.Errorf("save user code: %w", err)
	}
	return &StartResult{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: strings.TrimRight(verificationURI, "/") + "?user_code=" + userCode,
		ExpiresIn:       int(sessionTTL / time.Second),
		Interval:        defaultPoll,
	}, nil
}

func GetByUserCode(ctx context.Context, userCode string) (*Session, error) {
	deviceCode, err := redispool.Redis().Get(ctx, userCodeKey(userCode)).Result()
	if err != nil {
		return nil, err
	}
	return getSession(ctx, deviceCode)
}

func Approve(ctx context.Context, userCode string, uin, companyID uint, companyName string) error {
	deviceCode, err := redispool.Redis().Get(ctx, userCodeKey(userCode)).Result()
	if err != nil {
		return fmt.Errorf("authorization session not found: %w", err)
	}
	session, err := getSession(ctx, deviceCode)
	if err != nil {
		return err
	}
	if session.Status != "pending" {
		return fmt.Errorf("authorization session is %s", session.Status)
	}
	session.Status = "approved"
	session.UIN = uin
	session.CompanyID = companyID
	session.CompanyName = companyName
	remaining := time.Until(time.Unix(session.ExpiresAtUnix, 0))
	if remaining <= 0 {
		return fmt.Errorf("authorization session expired")
	}
	if err := redispool.Redis().Set(ctx, sessionKey(deviceCode), marshalSession(*session), remaining).Err(); err != nil {
		return fmt.Errorf("save approved authorization: %w", err)
	}
	return nil
}

func Deny(ctx context.Context, userCode string) error {
	deviceCode, err := redispool.Redis().Get(ctx, userCodeKey(userCode)).Result()
	if err != nil {
		return fmt.Errorf("authorization session not found: %w", err)
	}
	session, err := getSession(ctx, deviceCode)
	if err != nil {
		return err
	}
	if session.Status != "pending" {
		return fmt.Errorf("authorization session is %s", session.Status)
	}
	session.Status = "denied"
	remaining := time.Until(time.Unix(session.ExpiresAtUnix, 0))
	if remaining <= 0 {
		return fmt.Errorf("authorization session expired")
	}
	return redispool.Redis().Set(ctx, sessionKey(deviceCode), marshalSession(*session), remaining).Err()
}

func Poll(ctx context.Context, deviceCode string) (*PollResult, error) {
	session, err := getSession(ctx, deviceCode)
	if err != nil {
		if err == redis.Nil {
			return &PollResult{Status: "expired"}, nil
		}
		return nil, err
	}
	if time.Now().After(time.Unix(session.ExpiresAtUnix, 0)) {
		return &PollResult{Status: "expired"}, nil
	}
	if session.Status == "approved" {
		value, err := redispool.Redis().GetDel(ctx, sessionKey(deviceCode)).Result()
		if err == redis.Nil {
			return &PollResult{Status: "expired"}, nil
		}
		if err != nil {
			return nil, err
		}
		var consumed Session
		if err := unmarshalSession(value, &consumed); err != nil {
			return nil, err
		}
		_ = redispool.Redis().Del(ctx, userCodeKey(consumed.UserCode)).Err()
		key, err := apikey.CreatAPIKey(ctx, consumed.UIN, consumed.CompanyID, "CoreKG CLI", "corekg_cli", nil)
		if err != nil {
			return nil, fmt.Errorf("create cli API key: %w", err)
		}
		return &PollResult{Status: "approved", APIKey: key.APIKey, APIKeyID: key.ID, APIKeyPurpose: key.Purpose, UIN: consumed.UIN, CompanyID: consumed.CompanyID, CompanyName: consumed.CompanyName}, nil
	}
	return &PollResult{Status: session.Status}, nil
}

func sessionKey(deviceCode string) string {
	hash := sha256.Sum256([]byte(deviceCode))
	return "corekg:cli-auth:device:" + hex.EncodeToString(hash[:])
}

func userCodeKey(userCode string) string {
	return "corekg:cli-auth:user:" + strings.ToUpper(strings.TrimSpace(userCode))
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomUserCode(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = userCodeAlphabet[int(buf[i])%len(userCodeAlphabet)]
	}
	return string(buf), nil
}

func getSession(ctx context.Context, deviceCode string) (*Session, error) {
	value, err := redispool.Redis().Get(ctx, sessionKey(deviceCode)).Result()
	if err != nil {
		return nil, err
	}
	var session Session
	if err := unmarshalSession(value, &session); err != nil {
		return nil, err
	}
	return &session, nil
}
