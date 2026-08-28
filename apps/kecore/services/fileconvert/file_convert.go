package fileconvert

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/kecore/models/fs"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/settings"
	"github.com/ygpkg/yg-go/storage"
)

type convertConfig struct {
	URLPrefix string `yaml:"url_prefix"`
}

func getConvertServiceBaseURL() string {
	var cfg convertConfig
	if err := settings.GetYaml("corekg", "core_file_convert", &cfg); err != nil || cfg.URLPrefix == "" {
		return "http://127.0.0.1:8080/convert"
	}
	return cfg.URLPrefix
}

func ConvertFileIfNeeded(ctx *gin.Context, fileInfo *storage.FileInfo) (*storage.FileInfo, error) {
	for _, strategy := range convertStrategies {
		if strings.ToLower(fileInfo.FileExt) != strategy.SourceExt() {
			continue
		}

		shouldConvert, err := strategy.ShouldConvert(ctx, fileInfo)
		if err != nil {
			logs.ErrorContextf(ctx, "ConvertFileIfNeeded ShouldConvert error: %v", err)
			return nil, fmt.Errorf("判断转换失败: %v", err)
		}

		if !shouldConvert {
			continue
		}

		convertedFile, err := doConvert(ctx, fileInfo, strategy.TargetExt())
		if err != nil {
			return nil, err
		}

		t := time.Now()
		fileInfo.CompletedAt = &t
		fileInfo.Status = storage.FileStatusUploadSuccess
		if err := dbutil.Core().WithContext(ctx).Save(fileInfo).Error; err != nil {
			logs.ErrorContextf(ctx, "ConvertFileIfNeeded save original file status error: %v", err)
			return nil, fmt.Errorf("保存原文件上传状态失败: %v", err)
		}

		return convertedFile, nil
	}

	return fileInfo, nil
}

func doConvert(ctx *gin.Context, fileInfo *storage.FileInfo, targetExt string) (*storage.FileInfo, error) {
	originalData, err := downloadFromStorage(ctx, fileInfo.StoragePath)
	if err != nil {
		logs.ErrorContextf(ctx, "doConvert downloadFromStorage error: %v", err)
		return nil, fmt.Errorf("下载文件失败: %v", err)
	}

	targetType := strings.TrimPrefix(targetExt, ".")
	convertedData, err := callConvertService(ctx, originalData, fileInfo.Filename, targetType)
	if err != nil {
		logs.ErrorContextf(ctx, "doConvert callConvertService error: %v", err)
		return nil, fmt.Errorf("转换失败: %v", err)
	}

	newFileInfo, err := reuploadConvertedFile(ctx, fileInfo, convertedData, targetExt)
	if err != nil {
		logs.ErrorContextf(ctx, "doConvert reuploadConvertedFile error: %v", err)
		return nil, fmt.Errorf("重新上传失败: %v", err)
	}

	return newFileInfo, nil
}

func downloadFromStorage(ctx *gin.Context, storagePath string) ([]byte, error) {
	storager, err := fs.GetForestStorage()
	if err != nil {
		return nil, err
	}

	reader, err := storager.ReadFile(storagePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func callConvertService(ctx *gin.Context, fileData []byte, filename, targetType string) ([]byte, error) {
	baseURL := getConvertServiceBaseURL()
	url := fmt.Sprintf("%s/%s", baseURL, targetType)

	req, err := newConvertUploadRequest(requestContext(ctx), url, filename, fileData)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("转换服务返回错误: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func requestContext(ctx *gin.Context) context.Context {
	if ctx != nil && ctx.Request != nil {
		return ctx.Request.Context()
	}
	return context.Background()
}

func newConvertUploadRequest(ctx context.Context, url, filename string, fileData []byte) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if filename == "" {
		filename = "file"
	} else {
		filename = filepath.Base(filename)
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func reuploadConvertedFile(ctx *gin.Context, original *storage.FileInfo, data []byte, targetExt string) (*storage.FileInfo, error) {
	originalName := strings.TrimSuffix(original.Filename, filepath.Ext(original.Filename))
	newFilename := originalName + targetExt

	originalPathWithoutExt := strings.TrimSuffix(original.StoragePath, filepath.Ext(original.StoragePath))
	newStoragePath := originalPathWithoutExt + targetExt

	storager, err := fs.GetForestStorage()
	if err != nil {
		return nil, err
	}

	newFileInfo := &storage.FileInfo{
		CompanyID:   original.CompanyID,
		Uin:         original.Uin,
		Purpose:     original.Purpose,
		Filename:    newFilename,
		FileExt:     targetExt,
		MIMEType:    original.MIMEType,
		Size:        int64(len(data)),
		Status:      original.Status,
		Extra:       original.Extra,
		StoragePath: newStoragePath,
	}

	if err := storager.Save(ctx, newFileInfo, bytes.NewReader(data)); err != nil {
		return nil, err
	}

	publicURL := storager.GetPublicURL(newStoragePath, false)
	newFileInfo.PublicURL = publicURL

	if err := dbutil.Core().WithContext(ctx).Create(newFileInfo).Error; err != nil {
		return nil, err
	}

	return newFileInfo, nil
}
