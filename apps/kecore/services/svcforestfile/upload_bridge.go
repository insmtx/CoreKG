package svcforestfile

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/kecore/internal/dto/dtoforestfile"
	"github.com/insmtx/corekg/apps/ketask/models/ragtask"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/ygpkg/yg-go/apis/runtime"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/storage"
)

type APIErrorResponse struct {
	Code    uint32
	Message string
}

type APIUploadFileItem struct {
	Filename    string
	Hash        string
	Size        int64
	SplitConfig *ragtask.SplitConfig
	ContentType string
}

type APIPreUploadFileRequest struct {
	ForestID uint
	ParentID uint
	Files    []APIUploadFileItem
}

type APIPreUploadMultipartConfig struct {
	Enabled   bool
	ChunkSize int64
	PartCount int
}

type APIPreUploadFileResponseItem struct {
	UploadID   uint
	Hash       string
	Exists     bool
	Multipart  *APIPreUploadMultipartConfig
	UploadURLs map[int]string
}

type APIUploadPart struct {
	PartNumber int
	ETag       string
}

type APIUploadFileCallBackRequest struct {
	ForestID uint
	UploadID uint
	Filename string
	Hash     string
	Parts    []APIUploadPart
}

// PreUploadFileForAPI 提供给外部编排层调用，复用内部预上传逻辑并隔离 internal DTO。
func PreUploadFileForAPI(ctx *gin.Context, req *APIPreUploadFileRequest) ([]*APIPreUploadFileResponseItem, *APIErrorResponse) {
	dtoReq := &dtoforestfile.PreUploadFileRequest{}
	dtoReq.Request.ForestID = req.ForestID
	dtoReq.Request.ParentID = req.ParentID
	dtoReq.Request.Files = make([]dtoforestfile.UploadFileItem, 0, len(req.Files))
	for _, file := range req.Files {
		dtoReq.Request.Files = append(dtoReq.Request.Files, dtoforestfile.UploadFileItem{
			Filename:    file.Filename,
			Hash:        file.Hash,
			Size:        file.Size,
			SplitConfig: file.SplitConfig,
			ContentType: file.ContentType,
		})
	}

	files, errInfo := preUploadFile(ctx, dtoReq, preUploadForServer)
	if errInfo != nil {
		return nil, &APIErrorResponse{Code: errInfo.Code, Message: errInfo.Message}
	}

	resp := make([]*APIPreUploadFileResponseItem, 0, len(files))
	for _, file := range files {
		item := &APIPreUploadFileResponseItem{
			UploadID:   file.UploadID,
			Hash:       file.Hash,
			Exists:     file.Exists,
			UploadURLs: file.UploadURLs,
		}
		if file.Multipart != nil {
			item.Multipart = &APIPreUploadMultipartConfig{
				Enabled:   file.Multipart.Enabled,
				ChunkSize: file.Multipart.ChunkSize,
				PartCount: file.Multipart.PartCount,
			}
		}
		resp = append(resp, item)
	}

	return resp, nil
}

// UploadFileCallBackForAPI 提供给外部编排层调用，复用内部上传完成回调逻辑并隔离 internal DTO。
func UploadFileCallBackForAPI(ctx *gin.Context, req *APIUploadFileCallBackRequest) (*uint, *APIErrorResponse) {
	dtoReq := &dtoforestfile.UploadFileCallBackRequest{}
	dtoReq.Request.ForestID = req.ForestID
	dtoReq.Request.UploadID = req.UploadID
	dtoReq.Request.Filename = req.Filename
	dtoReq.Request.Hash = req.Hash
	dtoReq.Request.Parts = make([]struct {
		PartNumber int    `json:"part_number"`
		ETag       string `json:"etag"`
	}, 0, len(req.Parts))
	for _, part := range req.Parts {
		dtoReq.Request.Parts = append(dtoReq.Request.Parts, struct {
			PartNumber int    `json:"part_number"`
			ETag       string `json:"etag"`
		}{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}

	fileID, errInfo := UploadFileCallBack(ctx, dtoReq)
	if errInfo != nil {
		return nil, &APIErrorResponse{Code: errInfo.Code, Message: errInfo.Message}
	}

	return fileID, nil
}

// PrepareCoreFileForDirectUpload 将预上传生成的记录调整为单文件直传可用状态，供外部服务端直写存储后再回调。
func PrepareCoreFileForDirectUpload(ctx *gin.Context, uploadID uint) (*storage.FileInfo, *APIErrorResponse) {
	coreFile, err := storage.GetFileByID(dbutil.Core(), uploadID)
	if err != nil {
		return nil, &APIErrorResponse{Code: 500, Message: "kecore_query_file_info_failed"}
	}
	if runtime.Uin(ctx) != coreFile.Uin {
		return nil, &APIErrorResponse{Code: 500, Message: "kecore_upload_user_error"}
	}
	if coreFile.Status != storage.FileStatusUploading && coreFile.Status != storage.FileStatusUploadSuccess {
		return nil, &APIErrorResponse{Code: 500, Message: "kecore_file_status_error"}
	}

	// 若预上传阶段已创建分片上传任务，这里先显式取消对象存储侧的 multipart 状态。
	if coreFile.UploadS3ID != "" {
		uploadStorager, err := getInternalForestStorage()
		if err != nil {
			return nil, &APIErrorResponse{Code: 500, Message: "kecore_get_upload_storager_failed"}
		}

		err = uploadStorager.AbortMultipartUpload(ctx, &storage.AbortMultipartUploadInput{
			StoragePath: &coreFile.StoragePath,
			UploadID:    &coreFile.UploadS3ID,
		})
		if err != nil && !isIgnorableAbortMultipartErr(err) {
			logs.ErrorContextf(ctx, "forest: PrepareCoreFileForDirectUpload abort multipart failed, upload_id = %s, err = %v", coreFile.UploadS3ID, err)
			return nil, &APIErrorResponse{Code: 500, Message: "kecore_abort_upload_failed"}
		}
	}

	coreFile.UploadS3ID = ""
	coreFile.UploadChunkTotal = 1
	if coreFile.Size > 0 {
		coreFile.UploadChunkSize = coreFile.Size
	}
	coreFile.UploadedChunks = nil

	if err := dbutil.Core().WithContext(ctx).Save(coreFile).Error; err != nil {
		return nil, &APIErrorResponse{Code: 500, Message: "kecore_create_file_failed"}
	}

	return coreFile, nil
}

func isIgnorableAbortMultipartErr(err error) bool {
	if err == nil {
		return true
	}

	msg := err.Error()
	return strings.Contains(msg, "NoSuchUpload") ||
		strings.Contains(msg, "does not exist")
}
