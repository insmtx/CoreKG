package svcforestfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/account/models/user"
	"github.com/insmtx/corekg/apps/kecore/internal/dto/dtoforestfile"
	"github.com/insmtx/corekg/apps/kecore/mds"
	"github.com/insmtx/corekg/apps/kecore/models/coretask"
	"github.com/insmtx/corekg/apps/kecore/models/decoupler"
	"github.com/insmtx/corekg/apps/kecore/models/forest"
	"github.com/insmtx/corekg/apps/kecore/models/foresttype"
	"github.com/insmtx/corekg/apps/kecore/models/fs"
	"github.com/insmtx/corekg/apps/kecore/services/fileconvert"
	"github.com/insmtx/corekg/apps/kecore/services/messagecenter"
	"github.com/insmtx/corekg/apps/kecore/services/svcexcelforest"
	"github.com/insmtx/corekg/pkgs/global"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/ygpkg/yg-go/apis/errcode"
	"github.com/ygpkg/yg-go/apis/runtime"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/storage"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type preUploadMode uint8

const (
	preUploadForBrowser preUploadMode = iota
	preUploadForServer
)

// 文件上传
func PreUploadFile(ctx *gin.Context, req *dtoforestfile.PreUploadFileRequest) (preUploadFiles []*dtoforestfile.PreUploadFileResponseItem, errInfo *dtoforestfile.ErrorResponse) {
	return preUploadFile(ctx, req, preUploadForBrowser)
}

func preUploadFile(ctx *gin.Context, req *dtoforestfile.PreUploadFileRequest, mode preUploadMode) (preUploadFiles []*dtoforestfile.PreUploadFileResponseItem, errInfo *dtoforestfile.ErrorResponse) {
	var (
		parent    *foresttype.KnownowForestFile
		err       error
		uin       = runtime.Uin(ctx)
		companyID = runtime.CompanyID(ctx)
	)
	// 获取父目录信息
	if req.Request.ParentID > 0 {
		parent, err = forest.GetForestFileByID(req.Request.ParentID)
		if err != nil {
			// 查询父级目录失败
			logs.ErrorContextf(ctx, "forest: PreUploadFile failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_query_parent_failed"}
		}
	}

	uploadStorager, err := getInternalForestStorage()
	if err != nil {
		// 获取上传存储失败
		logs.ErrorContextf(ctx, "forest: PreUploadFile getUploadStorager failed,err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_upload_storager_failed"}
	}
	var publicPresigner presignedURLGenerator
	if mode == preUploadForBrowser {
		publicPresigner, err = getPublicForestPresigner()
		if err != nil {
			logs.ErrorContextf(ctx, "forest: PreUploadFile get public presigner failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_upload_storager_failed"}
		}
	}

	method := http.MethodPut
	presignedUrlCount := 0
	respFiles := []*dtoforestfile.PreUploadFileResponseItem{}
	// 遍历多个文件处理 TODO 后续优化批量处理行为
	for _, file := range req.Request.Files {
		// coreFile
		fi, err := buildUploadFile(ctx, req.Request.ForestID, &file, parent)
		if err != nil {
			// 构建文件信息异常
			logs.ErrorContextf(ctx, "forest: PreUploadFile buildUploadFile failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
		}
		// 计算hash命中
		incomingFile, err := storage.NewFileQuery(dbutil.Core()).Hash(fi.Hash).Status(storage.FileStatusNormal).Last()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logs.ErrorContextf(ctx, "forest: PreUploadFile GetFileByHash failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_file_failed"}
		}

		// =======================IF HAS VIEWABLE FILE=========================
		go func(c context.Context) {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logs.InfoContextf(c, "No file found for hash: %v", fi.Hash)
				return
			}
			ui, err := user.GetUserIdentificationByUIN(c, uin)
			if err != nil {
				logs.ErrorContextf(c, "GetUserIdentificationByUIN(uin:%v) faild err: %v", uin, err)
				return
			}

			ff, err := forest.GetForestFileByFileID(c, incomingFile.ID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					logs.InfoContextf(c, "No forest file found for coreFileID: %v", incomingFile.ID)
					return
				}
				logs.ErrorContextf(c, "GetForestFileByFileID(coreFileID:%v) faild err: %v", incomingFile.ID, err)
				return
			}
			forestEntity, err := forest.GetForestByID(c, ff.ForestID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					logs.InfoContextf(c, "No forest found for forestID: %v", ff.ForestID)
					return
				}
				logs.ErrorContextf(c, "GetForestByID(forestID:%v) faild err: %v", ff.ForestID, err)
				return
			}

			if !mds.CanViewForest(c, ff.ForestID, uin, companyID) {
				logs.InfoContextf(c, "Has no view permission for forest file: %+v", ff.ID)
				return
			}

			if mds.IsBanResource(c, uin, ff.ForestID, foresttype.ResourceTypeForest) {
				logs.InfoContextf(c, "Has ban permission for forest file: %+v", ff.ID)
				return
			}

			_, err = messagecenter.NewMessage().SendMessage(c, &messagecenter.SendMessageReq{
				CompanyID:    companyID,
				UserID:       ui.UserID,
				Uin:          uin,
				TemplateName: foresttype.MessageTemplateNameAlreadyUploadSameFile,
				SourceType:   foresttype.MessageSourceTypeForestFile,
				SourceID:     ff.ID,
				MessageParams: map[string]string{
					"newFileName": fi.Filename,
					"oldFileName": incomingFile.Filename,
					"forestName":  forestEntity.Name,
				},
			})
			if err != nil {
				logs.ErrorContextf(c, "SendMessage failed err: %v", err)
			}
		}(ctx)

		// =======================DONE=========================
		if isInstantUploadMatch(fi, incomingFile) {
			// 秒传命中
			t := time.Now()
			err := updateFileIfMatch(fi, incomingFile)
			if err != nil {
				logs.ErrorContextf(ctx, "forest: PreUploadFile updateFileIfMatch failed,err = %v", err)
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
			}
			fi.CompletedAt = &t
			fi.PublicURL = fs.Forest.GetPublicURL(fi.StoragePath, false)
			fi.Status = storage.FileStatusUploadSuccess

			if err := fs.CreateFileInfo(fi); err != nil {
				logs.ErrorContextf(ctx, "forest: PreUploadFile CreateFileInfo failed,err = %v", err)
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
			}

			respFiles = append(respFiles, &dtoforestfile.PreUploadFileResponseItem{
				UploadID: fi.ID,
				Hash:     file.Hash,
				Exists:   true,
			})
			continue
		}

		fi.PublicURL = fs.Forest.GetPublicURL(fi.StoragePath, false)
		// 计算分片上传配置
		partSize, partCount := CalcOptimalPart(fi.Size)
		if mode == preUploadForServer {
			partSize, partCount = fi.Size, 1
		}
		fi.UploadChunkSize = partSize
		fi.UploadChunkTotal = partCount
		fi.Exists = false

		enableMultipart := fi.UploadChunkTotal > 1
		if enableMultipart {
			upId, err := uploadStorager.CreateMultipartUpload(ctx, &storage.CreateMultipartUploadInput{
				StoragePath: &fi.StoragePath,
				ContentType: &file.ContentType,
			})
			if err != nil {
				logs.ErrorContextf(ctx, "forest: PreUploadFile CreateMultipartUpload failed,err = %v", err)
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
			}
			fi.UploadS3ID = *upId
		}

		if err := fs.CreateFileInfo(fi); err != nil {
			logs.ErrorContextf(ctx, "forest: PreUploadFile CreateFileInfo failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
		}

		// 生成预签名 URL
		uploadPartUrlMap := map[int]string{}
		if mode == preUploadForBrowser && presignedUrlCount < 20 {
			presignedUrlNum := 1
			if enableMultipart {
				presignedUrlNum = fi.UploadChunkTotal
				if presignedUrlNum > 5 {
					presignedUrlNum = 5 // 单文件一次最多生成 5 个
				}
			}
			// 统一生成预签名 URL
			for i := 1; i <= presignedUrlNum; i++ {
				input := &storage.GeneratePresignedURLInput{
					Method:      &method,
					StoragePath: &fi.StoragePath,
					ContentType: &file.ContentType,
				}
				if enableMultipart {
					input.UploadID = &fi.UploadS3ID
					input.PartNumber = &i
					// TODO 验证hash与len
				}
				urlPtr, err := publicPresigner.GeneratePresignedURL(ctx, input)
				if err != nil {
					logs.ErrorContextf(ctx, "get presigned url error: %v", err)
					return nil, &dtoforestfile.ErrorResponse{
						Code:    errcode.ErrCode_InternalError,
						Message: "kecore_get_presigned_url_failed",
					}
				}
				uploadPartUrlMap[i] = *urlPtr
				presignedUrlCount++
			}
		}

		respFiles = append(respFiles, &dtoforestfile.PreUploadFileResponseItem{
			UploadID: fi.ID,
			Hash:     file.Hash,
			Exists:   false,
			Multipart: &dtoforestfile.PreUploadMultipartConfig{
				Enabled:   enableMultipart,
				ChunkSize: fi.UploadChunkSize,
				PartCount: fi.UploadChunkTotal,
			},
			UploadURLs: uploadPartUrlMap,
		})
	}

	return respFiles, nil
}

// 上传成功回调
func UploadFileCallBack(ctx *gin.Context, req *dtoforestfile.UploadFileCallBackRequest) (fileID *uint, errInfo *dtoforestfile.ErrorResponse) {
	// uploadId 获取上传记录
	coreFile, err := storage.GetFileByID(dbutil.Core(), req.Request.UploadID)
	if err != nil {
		logs.ErrorContextf(ctx, "forest: UploadFileCallBack failed,err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_query_file_info_failed"}
	}
	if runtime.Uin(ctx) != coreFile.Uin {
		// 上传用户异常
		logs.ErrorContextf(ctx, "forest: UploadFileCallBack failed,err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_upload_user_error"}
	}
	if coreFile.Status != storage.FileStatusUploading && coreFile.Status != storage.FileStatusUploadSuccess {
		// 文件状态异常
		logs.ErrorContextf(ctx, "forest: UploadFileCallBack failed,err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_file_status_error"}
	}
	// TODO 更多校验策略

	if coreFile.Status == storage.FileStatusUploading {
		// 判断是否分片上传
		isMultipart := coreFile.UploadS3ID != ""
		if isMultipart {
			// 合并分片
			if len(req.Request.Parts) != coreFile.UploadChunkTotal {
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_BadRequest, Message: "parts_count_error"}
			}
			uploadStorager, err := getInternalForestStorage()
			if err != nil {
				logs.ErrorContextf(ctx, "forest: UploadFileCallBack failed,err = %v", err)
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_upload_storager_failed"}
			}
			completedParts := make([]types.CompletedPart, len(req.Request.Parts))
			for i, part := range req.Request.Parts {
				pn := int32(part.PartNumber)
				etag := part.ETag
				completedParts[i] = types.CompletedPart{
					PartNumber: &pn,
					ETag:       &etag,
				}
			}
			err = uploadStorager.CompleteMultipartUpload(ctx, &storage.CompleteMultipartUploadInput{
				StoragePath: &coreFile.StoragePath,
				UploadID:    &coreFile.UploadS3ID,
				Parts: &types.CompletedMultipartUpload{
					Parts: completedParts,
				},
			})
			if err != nil {
				logs.ErrorContextf(ctx, "forest: UploadFileCallBack failed,err = %v", err)
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_commit_upload_part_failed"}
			}
		}
		// 直传情况当作s3已经存储

		coreFile, err = fileconvert.ConvertFileIfNeeded(ctx, coreFile)
		if err != nil {
			logs.ErrorContextf(ctx, "forest: UploadFileCallBack ConvertFileIfNeeded failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_file_convert_failed"}
		}
		if coreFile == nil {
			logs.ErrorContextf(ctx, "forest: UploadFileCallBack ConvertFileIfNeeded returned nil file")
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_file_convert_failed"}
		}

		t := time.Now()
		coreFile.CompletedAt = &t
		coreFile.Status = storage.FileStatusUploadSuccess

		// 处理预览文件
		var previewSupport bool
		var previewFileEntity *storage.FileInfo
		if coreFile.FileExt == global.FileExtCSV {
			// TODO csv 生成预览临时处理
			previewFileEntity, err = generatePreviewFile(ctx, coreFile)
			if err != nil {
				logs.ErrorContextf(ctx, "forest: UploadFileCallBack generatePreviewFile failed,err = %v", err)
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
			}
			previewSupport = previewFileEntity != nil && previewFileEntity.ID != coreFile.ID
		} else {
			previewSupport, previewFileEntity = buildPreviewFileEntity(ctx, coreFile)
		}
		if previewSupport {
			if err := fs.CreateFileInfo(previewFileEntity); err != nil {
				logs.ErrorContextf(ctx, "forest: UploadFileCallBack CreatePreviewFileInfo failed %v", err)
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
			}
		}

		extra, err := parseExtra(coreFile.Extra)
		if err != nil {
			logs.ErrorContextf(ctx, "forest: UploadFileCallBack parseExtra failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
		}
		extra.PreviewFileID = previewFileEntity.ID
		extra.PreviewExt = previewFileEntity.FileExt

		bytes, err := json.Marshal(extra)
		if err != nil {
			logs.ErrorContextf(ctx, "forest: UploadFileCallBack Marshal failed,err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
		}
		coreFile.Extra = datatypes.JSON(bytes)
	}

	if err != nil {
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_file_convert_failed"}
	}

	// 创建知识库文件信息
	foresttFile, err := buildForestFile(ctx, coreFile)
	if err != nil {
		logs.ErrorContextf(ctx, "forest: UploadFileCallBack buildForestFile failed,err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
	}

	foresttFile.Status = foresttype.FileStatusNormal
	coreFile.Status = storage.FileStatusNormal

	err = dbutil.Knownow().Transaction(func(tx *gorm.DB) error {
		if err := dbutil.Core().WithContext(ctx).Save(coreFile).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Create(foresttFile).Error
	})
	if err != nil {
		logs.ErrorContextf(ctx, "forest: UploadFileCallBack CreateFileInfo failed %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_file_failed"}
	}
	// 校验知识库
	forestEntity, err := forest.NewForestDao().GetByID(ctx, foresttFile.ForestID)
	if err != nil {
		logs.ErrorContextf(ctx, "forest: GetByID failed,err = %v ,foresttFile:%+v", err.Error(), foresttFile)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_forest_failed"}
	}
	isTableFile := foresttFile.Ext == global.FileExtXLSX || foresttFile.Ext == global.FileExtCSV
	shouldHandleAsExcel := forestEntity.DataSourceType == foresttype.ForestDataSourceExcel ||
		(forestEntity.ForestType == foresttype.ForestTypeFile && isTableFile)
	switch {
	// excel 知识库或表格文件触发解析任务
	case shouldHandleAsExcel:
		go func() {
			defer func() {
				if err := recover(); err != nil {
					logs.ErrorContextf(ctx, "[UploadFileCallBack] svcexcelforest.AnalyzeXlsx panic: %v", err)
				}
			}()

			if foresttFile.Ext != global.FileExtXLSX && foresttFile.Ext != global.FileExtCSV {
				return
			}
			updateMap := map[string]interface{}{
				"preview_able": foresttype.PreViewAbleStatusAccept,
			}
			for _, v := range svcexcelforest.TaskStatusFields {
				updateMap[v] = foresttype.TaskStatusSuccess
			}

			forestUpdateMap := map[string]interface{}{
				"knowledge_status": foresttype.TaskStatusSuccess,
			}

			dbutil.Knownow().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				if err = forest.NewForestFileDao().UpdateMap(ctx, foresttFile.ID, updateMap); err != nil {
					logs.ErrorContextf(ctx, "[UploadFileCallBack] forest file updateMap failed, err = %v", err)
					return err
				}
				if err = forest.NewForestDao().UpdateMap(ctx, foresttFile.ForestID, forestUpdateMap); err != nil {
					logs.ErrorContextf(ctx, "[UploadFileCallBack] update forest success error: %v", err)
					return err
				}
				return nil
			})

		}()
		return &foresttFile.ID, nil
	default:
		// 生产任务
		if foresttFile.ParseStatus != foresttype.TaskStatusUnsupported {
			if err = coretask.CreateForestTask(ctx, foresttFile); err != nil {
				logs.ErrorContextf(ctx, "forest: CreateForestTask failed,err = %v", err.Error())
				// 创建任务失败
				return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_create_task_failed"}
			}
		}
	}

	return &foresttFile.ID, nil
}

// 上传预签名url重新生成
func RenewUploadUrl(ctx *gin.Context, req *dtoforestfile.RenewUploadUrlRequest) (renewedUrls *map[int]string, errInfo *dtoforestfile.ErrorResponse) {
	expiredParts := req.Request.ExpiredParts
	coreUploadFile, err := storage.GetFileByID(dbutil.Core(), req.Request.UploadID)
	if err != nil {
		logs.ErrorContextf(ctx, "forest: AbortUpload GetFileByID failed, err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_file_info_failed"}
	}
	if runtime.Uin(ctx) != coreUploadFile.Uin {
		// 检查当前用户是否为文件上传者，只有上传者才能重新生成预签名
		logs.ErrorContextf(ctx, "forest: RenewUploadUrl permission denied, uin = %d", runtime.Uin(ctx))
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_renew_upload_url_permission_denied"}
	}
	if coreUploadFile.Status != storage.FileStatusUploading {
		// 判断文件上传状态，仅上传中去生成预签名
		logs.ErrorContextf(ctx, "forest: RenewUploadUrl file status not uploading, status = %d", coreUploadFile.Status)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_BadRequest, Message: "kecore_renew_upload_url_status_denied"}
	}

	uploadStorager, err := getPublicForestPresigner()
	if err != nil {
		logs.ErrorContextf(ctx, "forest: RenewUploadUrl GetUploadStorager failed, err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_upload_storager_failed"}
	}
	method := http.MethodPut
	urls := make(map[int]string)

	if len(expiredParts) == 0 {
		// 直传 生成预签名
		url, err := uploadStorager.GeneratePresignedURL(ctx, &storage.GeneratePresignedURLInput{
			Method:      &method,
			StoragePath: &coreUploadFile.StoragePath,
		})
		if err != nil {
			logs.ErrorContextf(ctx, "forest: RenewUploadUrl GeneratePresignedURL failed, err = %v", err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_presigned_url_failed"}
		}
		urls[1] = *url
		return &urls, nil
	}

	max := expiredParts[0]
	for _, n := range expiredParts[1:] {
		if n > max {
			max = n
		}
	}
	if max > coreUploadFile.UploadChunkTotal {
		// 校验分片序号是否超出范围
		logs.ErrorContextf(ctx, "forest: RenewUploadUrl expired part number out of range, max = %d, parts = %d", max, coreUploadFile.UploadChunkTotal)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_BadRequest, Message: "kecore_renew_upload_url_part_number_out_of_range"}
	}

	for _, partNumber := range req.Request.ExpiredParts {
		url, err := uploadStorager.GeneratePresignedURL(ctx, &storage.GeneratePresignedURLInput{
			Method:      &method,
			StoragePath: &coreUploadFile.StoragePath,
			UploadID:    &coreUploadFile.UploadS3ID,
			PartNumber:  &partNumber,
		})
		if err != nil {
			logs.ErrorContextf(ctx, "forest: RenewUploadUrl GeneratePresignedURL failed, partNumber = %d, err = %v", partNumber, err)
			return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_presigned_url_failed"}
		}
		urls[partNumber] = *url
	}

	// 更新文件信息
	coreUploadFile.RenewCount++
	var chunks []storage.UploadedChunk
	for _, part := range req.Request.CompletedParts {
		chunks = append(chunks, storage.UploadedChunk{
			PartNumber: part,
		})
	}
	coreUploadFile.UploadedChunks = chunks

	if err := dbutil.Core().Save(coreUploadFile).Error; err != nil {
		logs.ErrorContextf(ctx, "forest: RenewUploadUrl save failed, err = %v", err)
		return nil, &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_renew_upload_url_failed"}
	}

	return &urls, nil
}

// 取消上传
func AbortUpload(ctx *gin.Context, req *dtoforestfile.AbortUploadRequest) (errInfo *dtoforestfile.ErrorResponse) {
	// 根据UploadID获取文件信息
	fileInfo, err := storage.GetFileByID(dbutil.Core(), req.Request.UploadID)
	if err != nil {
		logs.ErrorContextf(ctx, "forest: AbortUpload GetFileByID failed, err = %v", err)
		return &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_get_file_info_failed"}
	}

	// 检查当前用户是否为文件上传者，只有上传者才能取消上传
	currentUin := runtime.Uin(ctx)
	if fileInfo.Uin != currentUin {
		logs.ErrorContextf(ctx, "forest: AbortUpload permission denied, file owner uin = %d, current uin = %d", fileInfo.Uin, currentUin)
		return &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_BadRequest, Message: "kecore_abort_upload_permission_denied"}
	}

	// 更新文件状态为已取消，并记录取消时间
	now := time.Now()
	fileInfo.Status = storage.FileStatusAborted
	fileInfo.AbortAt = &now

	// 保存更新
	if err := dbutil.Core().Save(fileInfo).Error; err != nil {
		logs.ErrorContextf(ctx, "forest: AbortUpload save failed, err = %v", err)
		return &dtoforestfile.ErrorResponse{Code: errcode.ErrCode_InternalError, Message: "kecore_abort_upload_failed"}
	}

	// 触发碎片清理，处理上传过程数据

	// TODO 判断分片，调用S3清理（也可设置策略自动清理）

	return nil
}

func generatePreviewFile(ctx *gin.Context, coreFile *storage.FileInfo) (*storage.FileInfo, error) {
	storagePath := coreFile.StoragePath
	f, err := fs.Forest.ReadFile(storagePath)
	if err != nil {
		logs.ErrorContextf(ctx, "forest: UploadFileCallBack ReadFile failed %v", err)
		return nil, err
	}

	var reader io.Reader
	var closeFunc func()
	var priviewExt string

	switch coreFile.FileExt {
	case ".ppt", ".pptx", ".doc", ".docx", ".ofd":
		pdf, err := decoupler.FileToPDF(ctx, f, coreFile.Filename)
		if err != nil {
			logs.ErrorContextf(ctx, "forest: FileToPDF failed %v", err)
			return nil, err
		}
		reader = pdf
		closeFunc = func() { pdf.Close() }
		priviewExt = ".pdf"
	case ".csv":
		excel, err := decoupler.CSVToExcel(ctx, f)
		if err != nil {
			logs.ErrorContextf(ctx, "forest: CSVToExcel failed %v", err)
			return nil, err
		}
		reader = excel
		priviewExt = ".xlsx"
	default:
		// 不需要生成预览，直接返回原文件
		return coreFile, nil
	}

	if closeFunc != nil {
		defer closeFunc()
	}

	// 生成新的存储路径
	storagePath = storage.GenerateFileStoragePath(foresttype.PurposeForestFile, coreFile.Uin, priviewExt)
	// 创建新的预览文件信息
	fi := &storage.FileInfo{
		CompanyID:   runtime.CompanyID(ctx),
		Uin:         runtime.Uin(ctx),
		Filename:    coreFile.Filename + priviewExt,
		Size:        coreFile.Size,
		FileExt:     priviewExt,
		StoragePath: storagePath,
	}
	// 保存文件
	if err := fs.Forest.Save(ctx, fi, reader); err != nil {
		logs.ErrorContextf(ctx, "forest: Save failed %v", err)
		return nil, err
	}
	// 获取公开 URL
	fi.PublicURL = fs.Forest.GetPublicURL(fi.StoragePath, false)
	return fi, nil
}

func buildPreviewFileEntity(ctx *gin.Context, coreFile *storage.FileInfo) (bool, *storage.FileInfo) {
	var priviewExt string
	switch coreFile.FileExt {
	case global.FileExtPPT, global.FileExtPPTX, global.FileExtDOC, global.FileExtDOCX, global.FileExtOFD:
		priviewExt = global.FileExtPDF
	case global.FileExtCSV:
		priviewExt = global.FileExtXLSX
	default:
		// 不需要生成预览，直接返回原文件
		return false, coreFile
	}
	storagePath := storage.GenerateFileStoragePath(foresttype.PurposeForestFile, coreFile.Uin, priviewExt)
	fileEntity := &storage.FileInfo{
		CompanyID:   runtime.CompanyID(ctx),
		Uin:         runtime.Uin(ctx),
		Filename:    coreFile.Filename + priviewExt,
		Size:        coreFile.Size,
		FileExt:     priviewExt,
		StoragePath: storagePath,
		PublicURL:   fs.Forest.GetPublicURL(storagePath, false),
		Status:      storage.FileStatusUploading,
	}

	return true, fileEntity
}

func buildUploadFile(ctx *gin.Context, forestID uint, fileInfo *dtoforestfile.UploadFileItem, parent *foresttype.KnownowForestFile) (*storage.FileInfo, error) {
	extraData := ExtraInfo{
		ForestID: forestID,
		FileConfig: foresttype.FileConfig{
			SplitConfig: fileInfo.SplitConfig,
		},
	}
	//层级目录
	if parent == nil {
		extraData.Depth = 1
	} else {
		extraData.ParentID = parent.ID
		extraData.ParentIDs = fmt.Sprintf("%s%d/", parent.ParentIDs, parent.ID)
		extraData.Depth = parent.Depth + 1
	}
	bytes, err := json.Marshal(extraData)
	if err != nil {
		return nil, err
	}

	fi := &storage.FileInfo{
		CompanyID: runtime.CompanyID(ctx),
		Uin:       runtime.Uin(ctx),
		Filename:  fileInfo.Filename,
		Size:      fileInfo.Size,
		FileExt:   strings.ToLower(filepath.Ext(fileInfo.Filename)),
		Status:    storage.FileStatusUploading,
		Extra:     datatypes.JSON(bytes),
		Hash:      fileInfo.Hash,
	}
	fi.StoragePath = storage.GenerateFileStoragePath(foresttype.PurposeForestFile, fi.Uin, fi.FileExt)

	return fi, nil
}

func updateFileIfMatch(coreFile *storage.FileInfo, matchFile *storage.FileInfo) error {
	coreFile.Exists = true
	coreExtra, err := parseExtra(coreFile.Extra)
	if err != nil {
		return err
	}
	matchExtra, err := parseExtra(matchFile.Extra)
	if err != nil {
		return err
	}
	// 秒传命中信息
	coreFile.StoragePath = matchFile.StoragePath
	coreExtra.PreviewFileID = matchExtra.PreviewFileID
	coreExtra.PreviewExt = matchExtra.PreviewExt

	bytes, err := json.Marshal(coreExtra)
	if err != nil {
		return err
	}
	coreFile.Extra = datatypes.JSON(bytes)
	return nil
}

func buildForestFile(ctx *gin.Context, coreFile *storage.FileInfo) (*foresttype.KnownowForestFile, error) {
	finfo := &foresttype.KnownowForestFile{
		CompanyID:  coreFile.CompanyID,
		Uin:        coreFile.Uin,
		IsDir:      -1,
		Name:       coreFile.Filename,
		Ext:        coreFile.FileExt,
		Size:       coreFile.Size,
		Status:     foresttype.FileStatusPending,
		CoreFileID: coreFile.ID,
	}

	info, err := parseExtra(coreFile.Extra)
	if err != nil {
		return nil, err
	}
	finfo.PriviewFileID = info.PreviewFileID
	finfo.PriviewExt = info.PreviewExt
	finfo.ForestID = info.ForestID
	finfo.FileConfig = info.FileConfig
	finfo.Depth = info.Depth
	finfo.ParentID = info.ParentID
	finfo.ParentIDs = info.ParentIDs

	isExist, err := forest.IsExistForestFile(finfo.ForestID, finfo.ParentID, finfo.Name)
	if err != nil {
		return nil, err
	}
	if isExist {
		logs.WarnContextf(ctx, "该文件已经存在,创建同名文件_timestep")
		finfo.Name = fmt.Sprintf("%v_%v%v", filepath.Base(finfo.Name), time.Now().Format("20060102_150405"), filepath.Ext(finfo.Name))
	}

	//不可预览文件类型标记
	if !forest.PreViewAble(finfo) {
		finfo.PreViewAble = foresttype.PreViewAbleStatusUnsupported
		finfo.ParseStatus = foresttype.TaskStatusUnsupported
		finfo.KnowledgeStatus = foresttype.TaskStatusUnsupported
		finfo.AnalysisStatus = foresttype.TaskStatusUnsupported
		finfo.GraphStatus = foresttype.TaskStatusUnsupported
	}
	return finfo, nil
}

// 动态计算上传分片大小
func CalcOptimalPart(fileSize int64) (partSize int64, partCount int) {
	const (
		MinPartSize int64 = 5 * 1024 * 1024   // 5MB
		MaxPartSize int64 = 500 * 1024 * 1024 // 500MB
		MaxParts    int64 = 10000
		MinFileSize int64 = 30 * 1024 * 1024 // 30MB，小文件直接 1 片
	)

	// 对于小文件，直接返回 1 片
	if fileSize <= MinFileSize {
		return fileSize, 1
	}

	// 根据文件大小动态确定目标分片数
	var targetParts int64
	switch {
	case fileSize <= 100*1024*1024: // <=100MB
		targetParts = 5 + fileSize/(10*1024*1024) // 小文件分片数 5~10
	case fileSize <= 1*1024*1024*1024: // <=1GB
		targetParts = 10 + fileSize/(100*1024*1024) // 中等文件 10~50
	case fileSize <= 10*1024*1024*1024: // <=10GB
		targetParts = 50 + fileSize/(500*1024*1024) // 大文件 50~200
	default:
		targetParts = 200 + fileSize/(1*1024*1024*1024) // 超大文件
	}

	// 计算分片大小
	partSize = (fileSize + targetParts - 1) / targetParts

	// 限制在最小/最大范围
	if partSize < MinPartSize {
		partSize = MinPartSize
	}
	if partSize > MaxPartSize {
		partSize = MaxPartSize
	}

	// 计算分片数量
	partCount64 := (fileSize + partSize - 1) / partSize
	if partCount64 > MaxParts {
		return 0, 0
	}
	partCount = int(partCount64)
	return partSize, partCount
}

func isInstantUploadMatch(existing, incoming *storage.FileInfo) bool {
	if existing == nil || incoming == nil {
		return false
	}
	// 文件必须处于正常状态
	if incoming.Status != storage.FileStatusNormal {
		return false
	}
	// 校验 Hash 是否一致
	if existing.Hash != incoming.Hash {
		return false
	}
	// 校验文件大小是否一致且非零
	if incoming.Size == 0 || existing.Size != incoming.Size {
		return false
	}
	// 确保是当前版本完整解析内容
	if len(incoming.Extra) == 0 {
		return false
	}
	info, err := parseExtra(incoming.Extra)
	if err != nil {
		return false
	}
	if info.PreviewFileID == 0 {
		return false
	}
	return true
}

type ExtraInfo struct {
	PreviewFileID uint                  `json:"previewFileId"`
	PreviewExt    string                `json:"previewExt"`
	ForestID      uint                  `json:"forestId"`
	FileConfig    foresttype.FileConfig `json:"fileConfig"`
	ParentID      uint                  `json:"parentId"`
	ParentIDs     string                `json:"parentIds"`
	Depth         int                   `json:"depth"`
}

func parseExtra(extra datatypes.JSON) (info *ExtraInfo, err error) {
	info = &ExtraInfo{}
	// 空 JSON 或 nil 直接返回空对象
	if len(extra) == 0 {
		return info, nil
	}

	if err := json.Unmarshal(extra, info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ExtraInfo: %w", err)
	}
	return info, nil
}
