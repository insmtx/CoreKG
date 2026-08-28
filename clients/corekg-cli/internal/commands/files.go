package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/output"
	"github.com/spf13/cobra"
)

type fileUploadOutput struct {
	KnowledgeBaseID uint   `json:"knowledge_base_id"`
	ForestFileID    uint   `json:"forest_file_id"`
	Name            string `json:"name"`
	ParentID        uint   `json:"parent_id"`
	Status          string `json:"status"`
}

func (a *app) fileCommand() *cobra.Command {
	command := &cobra.Command{Use: "file", Short: "Manage knowledge base files"}
	command.AddCommand(a.fileListCommand())
	command.AddCommand(a.fileUploadCommand())
	return command
}

func (a *app) fileListCommand() *cobra.Command {
	var selector string
	var offset, limit int
	var all bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List files in the selected knowledge base",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if offset < 0 || limit < 0 || limit > api.MaxPageSize {
				return clierr.Usage("invalid_pagination", fmt.Sprintf("offset must not be negative and limit must be between 0 and %d", api.MaxPageSize))
			}
			if all && (cmd.Flags().Changed("offset") || cmd.Flags().Changed("limit")) {
				return clierr.Usage("invalid_pagination", "--all cannot be combined with --offset or --limit")
			}
			active, err := a.loadActiveProfile(a.profileName)
			if err != nil {
				return err
			}
			forest, err := a.resolveKnowledgeBase(cmd.Context(), active, selector)
			if err != nil {
				return err
			}

			var page api.ForestFilePage
			if all {
				page, err = a.listAllFiles(cmd.Context(), active, forest.ForestID)
			} else {
				err = active.Client.DoJSON(cmd.Context(), active.Credential.APIKey, "keapi.ListFile", map[string]any{
					"forest_id": forest.ForestID,
					"offset":    offset,
					"limit":     limit,
				}, &page)
			}
			if err != nil {
				return clierr.Wrap("file_list_failed", err)
			}
			return a.writeFilePage(page)
		},
	}
	command.Flags().StringVar(&selector, "kb", "", "Knowledge base ID or exact name (default: selected knowledge base)")
	command.Flags().IntVar(&offset, "offset", 0, "Starting offset")
	command.Flags().IntVar(&limit, "limit", 0, fmt.Sprintf("Maximum number of files (up to %d)", api.MaxPageSize))
	command.Flags().BoolVar(&all, "all", false, "Fetch all files")
	return command
}

func (a *app) fileUploadCommand() *cobra.Command {
	var selector string
	var parentID uint
	var followSymlinks bool
	var wait bool
	var waitTimeout time.Duration
	var uploadTimeout time.Duration
	command := &cobra.Command{
		Use:   "upload PATH",
		Short: "Upload a file to the selected knowledge base",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("upload-timeout") && uploadTimeout <= 0 {
				return clierr.Usage("invalid_upload_timeout", "--upload-timeout must be positive")
			}
			if waitTimeout <= 0 {
				return clierr.Usage("invalid_wait_timeout", "--wait-timeout must be positive")
			}
			active, err := a.loadActiveProfileForOperation(a.profileName, 10*time.Minute, uploadTimeout)
			if err != nil {
				return err
			}
			forest, err := a.resolveKnowledgeBase(cmd.Context(), active, selector)
			if err != nil {
				return err
			}
			file, fileName, err := openUploadFile(args[0], followSymlinks)
			if err != nil {
				return err
			}
			defer file.Close()

			var uploaded api.UploadFileResult
			if err := uploadFile(cmd.Context(), active, forest.ForestID, parentID, fileName, file, &uploaded); err != nil {
				return clierr.Wrap("file_upload_failed", err)
			}
			if uploaded.ForestFileID == 0 {
				return clierr.New("file_upload_failed", "keapi.UploadFile returned an empty file ID")
			}

			result := fileUploadOutput{
				KnowledgeBaseID: forest.ForestID,
				ForestFileID:    uploaded.ForestFileID,
				Name:            fileName,
				ParentID:        parentID,
				Status:          "accepted",
			}
			if wait {
				fileInfo, waitErr := a.waitForFile(cmd.Context(), active, uploaded.ForestFileID, waitTimeout)
				if waitErr != nil {
					return fileWaitCLIError(forest.ForestID, uploaded.ForestFileID, waitErr)
				}
				result.Name = fileInfo.Name
				result.ParentID = fileInfo.ParentID
				result.Status = fileInfo.FileStatus
			}
			return a.writeFileUploadOutput(result)
		},
	}
	command.Flags().StringVar(&selector, "kb", "", "Knowledge base ID or exact name (default: selected knowledge base)")
	command.Flags().UintVar(&parentID, "parent-id", 0, "Parent directory ID (0 for the knowledge base root)")
	command.Flags().BoolVar(&followSymlinks, "follow-symlinks", false, "Allow uploading the target of a symbolic link")
	command.Flags().BoolVar(&wait, "wait", false, "Wait until file parsing and indexing finish")
	command.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "Maximum time to wait when --wait is used")
	command.Flags().DurationVar(&uploadTimeout, "upload-timeout", 0, "HTTP timeout for this upload operation (default: 10m or longer configured timeout)")
	return command
}

func (a *app) listAllFiles(ctx context.Context, active *activeProfile, forestID uint) (api.ForestFilePage, error) {
	page := api.ForestFilePage{Data: make([]api.ForestFile, 0)}
	for offset := 0; ; offset += api.MaxPageSize {
		var current api.ForestFilePage
		if err := active.Client.DoJSON(ctx, active.Credential.APIKey, "keapi.ListFile", map[string]any{
			"forest_id": forestID,
			"offset":    offset,
			"limit":     api.MaxPageSize,
		}, &current); err != nil {
			return api.ForestFilePage{}, err
		}
		page.Data = append(page.Data, current.Data...)
		page.Total = current.Total
		page.Offset = 0
		page.Limit = len(page.Data)
		if len(current.Data) == 0 || len(current.Data) < api.MaxPageSize || int64(offset+len(current.Data)) >= current.Total {
			return page, nil
		}
	}
}

func (a *app) writeFilePage(page api.ForestFilePage) error {
	format, err := a.format()
	if err != nil {
		return err
	}
	if format == "json" {
		return output.WriteJSON(a.out, page)
	}
	if format == "id" {
		ids := make([]string, 0, len(page.Data))
		for _, file := range page.Data {
			ids = append(ids, strconv.FormatUint(uint64(file.ForestFileID), 10))
		}
		return output.WriteNames(a.out, ids)
	}
	rows := make([][]string, 0, len(page.Data))
	for _, file := range page.Data {
		kind := "file"
		if file.IsDir == 1 {
			kind = "dir"
		}
		createdAt := ""
		if !file.CreatedAt.IsZero() {
			createdAt = file.CreatedAt.Format(time.RFC3339)
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(file.ForestFileID), 10),
			file.Name,
			kind,
			file.FileStatus,
			strconv.FormatInt(file.Size, 10),
			createdAt,
		})
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(a.out, "No files found.")
		return err
	}
	return output.WriteTable(a.out, []string{"ID", "NAME", "TYPE", "STATUS", "SIZE", "CREATED"}, rows)
}

func (a *app) writeFileUploadOutput(value fileUploadOutput) error {
	format, err := a.format()
	if err != nil {
		return err
	}
	if format == "json" {
		return output.WriteJSON(a.out, value)
	}
	if format == "id" {
		return output.WriteNames(a.out, []string{strconv.FormatUint(uint64(value.ForestFileID), 10)})
	}
	return output.WriteTable(a.out, []string{"KNOWLEDGE BASE ID", "FILE ID", "NAME", "PARENT ID", "STATUS"}, [][]string{{
		strconv.FormatUint(uint64(value.KnowledgeBaseID), 10),
		strconv.FormatUint(uint64(value.ForestFileID), 10),
		value.Name,
		strconv.FormatUint(uint64(value.ParentID), 10),
		value.Status,
	}})
}

func openUploadFile(path string, followSymlinks bool) (*os.File, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, "", clierr.Usage("invalid_file", "file path must not be empty")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", clierr.Usage("invalid_file", fmt.Sprintf("inspect %q: %v", path, err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if !followSymlinks {
			return nil, "", clierr.Usage("invalid_file", fmt.Sprintf("refusing to upload symbolic link %q; pass --follow-symlinks to allow it", path))
		}
		info, err = os.Stat(path)
		if err != nil {
			return nil, "", clierr.Usage("invalid_file", fmt.Sprintf("inspect symbolic-link target %q: %v", path, err))
		}
	}
	if !info.Mode().IsRegular() {
		return nil, "", clierr.Usage("invalid_file", fmt.Sprintf("%q is not a regular file", path))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", clierr.Usage("invalid_file", fmt.Sprintf("open %q: %v", path, err))
	}
	return file, filepath.Base(path), nil
}

func uploadFile(ctx context.Context, active *activeProfile, forestID, parentID uint, fileName string, file *os.File, result *api.UploadFileResult) error {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()
	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	writerErr := make(chan error, 1)
	go func() {
		part, err := multipartWriter.CreateFormFile("file", fileName)
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if err == nil {
			err = multipartWriter.WriteField("forest_id", strconv.FormatUint(uint64(forestID), 10))
		}
		if err == nil && parentID > 0 {
			err = multipartWriter.WriteField("parent_id", strconv.FormatUint(uint64(parentID), 10))
		}
		if err == nil {
			err = multipartWriter.Close()
		}
		if err != nil {
			_ = writer.CloseWithError(err)
			writerErr <- err
			return
		}
		_ = writer.Close()
		writerErr <- nil
	}()

	err := active.Client.DoMultipart(uploadCtx, active.Credential.APIKey, "keapi.UploadFile", contentType, reader, result)
	if err != nil {
		_ = reader.CloseWithError(err)
		cancel()
	}
	producerErr := <-writerErr
	if err != nil {
		return err
	}
	return producerErr
}

func (a *app) waitForFile(ctx context.Context, active *activeProfile, fileID uint, timeout time.Duration) (api.ForestFile, error) {
	return a.waitForFileWithInterval(ctx, active, fileID, timeout, 2*time.Second)
}

func (a *app) waitForFileWithInterval(ctx context.Context, active *activeProfile, fileID uint, timeout, interval time.Duration) (api.ForestFile, error) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		file, err := getFile(waitCtx, active, fileID)
		if err != nil {
			if !errors.Is(err, errFileNotFound) {
				return api.ForestFile{}, &fileWaitError{code: "file_wait_failed", status: "unknown", cause: err}
			}
			if err := waitForNextFilePoll(waitCtx, ticker); err != nil {
				return api.ForestFile{}, &fileWaitError{code: fileWaitErrorCode(err), status: "not_found", cause: fmt.Errorf("file %d did not become ready within %s: %w", fileID, timeout, err)}
			}
			continue
		}
		switch file.FileStatus {
		case "success":
			return file, nil
		case "fail", "unsupported":
			return api.ForestFile{}, &fileWaitError{code: "file_processing_failed", status: file.FileStatus, cause: fmt.Errorf("file %d finished with status %q", fileID, file.FileStatus)}
		}
		if err := waitForNextFilePoll(waitCtx, ticker); err != nil {
			return api.ForestFile{}, &fileWaitError{code: fileWaitErrorCode(err), status: file.FileStatus, cause: fmt.Errorf("file %d did not become ready within %s: %w", fileID, timeout, err)}
		}
	}
}

var errFileNotFound = errors.New("file not found")

type fileWaitError struct {
	code   string
	status string
	cause  error
}

func (e *fileWaitError) Error() string {
	return e.cause.Error()
}

func (e *fileWaitError) Unwrap() error {
	return e.cause
}

func waitForNextFilePoll(ctx context.Context, ticker *time.Ticker) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return nil
	}
}

func fileWaitErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "file_wait_timeout"
	}
	return "file_wait_failed"
}

func fileWaitCLIError(forestID, fileID uint, err error) error {
	details := map[string]any{
		"knowledge_base_id": forestID,
		"forest_file_id":    fileID,
	}
	code := "file_wait_failed"
	var waitErr *fileWaitError
	if errors.As(err, &waitErr) {
		code = waitErr.code
		details["file_status"] = waitErr.status
	}
	return clierr.WithDetails(code, fmt.Sprintf("knowledge base %d file %d: %s", forestID, fileID, err), details)
}

func getFile(ctx context.Context, active *activeProfile, fileID uint) (api.ForestFile, error) {
	var page api.ForestFilePage
	if err := active.Client.DoJSON(ctx, active.Credential.APIKey, "keapi.BatchGetFile", map[string]any{
		"forest_file_ids": []uint{fileID},
	}, &page); err != nil {
		return api.ForestFile{}, err
	}
	if len(page.Data) == 0 {
		return api.ForestFile{}, fmt.Errorf("%w: file %d", errFileNotFound, fileID)
	}
	return page.Data[0], nil
}
