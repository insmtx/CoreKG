package svcforestfile

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/insmtx/corekg/apps/kecore/models/forest"
	"github.com/insmtx/corekg/apps/kecore/models/foresttype"
	"github.com/insmtx/corekg/apps/kecore/models/keqa"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/storage"
)

var (
	ErrUserIDEmpty           = errors.New("user id empty")
	ErrGetFileFailed         = errors.New("get file failed")
	ErrQueryForestListFailed = errors.New("query forest list failed")
	ErrGetSourceFileFailed   = errors.New("get source file failed")
	ErrFileNotPreviewable    = errors.New("file not previewable")
	ErrGetFilePathFailed     = errors.New("get file path failed")
	ErrGetUploadConfigFailed = errors.New("get upload config failed")
	ErrParseURLFailed        = errors.New("parse url failed")
	ErrCreateStorageFailed   = errors.New("create storage failed")
	ErrGetPresignedURLFailed = errors.New("get presigned url failed")
	ErrGetFileURLFailed      = errors.New("get file url failed")
)

type ListFileRequest struct {
	Uin       uint
	CompanyID uint
	ForestID  uint
	ImageURL  string
	PageQuery apiobj.PageQuery
}

type PreviewFileByURLRequest struct {
	FileID     uint
	IsDownload bool
	Referer    string
}

func ListFile(ctx context.Context, req *ListFileRequest) (*forest.QueryForestFileResponse, error) {
	if req.Uin == 0 {
		return nil, ErrUserIDEmpty
	}

	query := req.PageQuery
	query.Uin = req.Uin
	query.CompanyID = req.CompanyID
	if req.ForestID > 0 {
		query.Filters = append(query.Filters, apiobj.Filter{
			Field: "forest_id",
			Value: []string{fmt.Sprintf("%d", req.ForestID)},
		})
	}

	if req.ImageURL != "" {
		fileList, err := keqa.ImageQueryFile(ctx, req.Uin, req.ForestID, req.ImageURL)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGetFileFailed, err)
		}
		out := &forest.QueryForestFileResponse{}
		out.Response.Total = int64(len(fileList))
		out.Response.Data = make([]*forest.File, 0, len(fileList))
		for _, file := range fileList {
			status, progress := file.CalculateCompletionPercentage()
			out.Response.Data = append(out.Response.Data, &forest.File{
				KnownowForestFile: *file,
				FileStatus:        status,
				FileProgress:      progress,
			})
		}
		return out, nil
	}

	out, err := forest.QueryForestFile(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryForestListFailed, err)
	}
	return out, nil
}

func PreviewFileByURL(ctx context.Context, req *PreviewFileByURLRequest) (string, error) {
	file, err := forest.GetForestFileByID(req.FileID)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrGetSourceFileFailed, err)
	}
	if file.PreViewAble != foresttype.PreViewAbleStatusAccept {
		return "", ErrFileNotPreviewable
	}

	var filePath *string
	if req.IsDownload {
		filePath, err = file.GetForestFilePath()
	} else {
		filePath, err = file.GetForestPriviewFilePath()
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrGetFilePathFailed, err)
	}

	presigner, err := getPublicForestPresigner()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCreateStorageFailed, err)
	}
	method := http.MethodGet
	url, err := presigner.GeneratePresignedURL(ctx, &storage.GeneratePresignedURLInput{
		Method:      &method,
		StoragePath: filePath,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrGetPresignedURLFailed, err)
	}
	return *url, nil
}
