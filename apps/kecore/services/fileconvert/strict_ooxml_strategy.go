package fileconvert

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/kecore/models/fs"
	"github.com/insmtx/corekg/pkgs/global"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/storage"
)

type StrictOOXMLStrategy struct{}

const (
	strictSpreadsheetMLNamespace       = "http://purl.oclc.org/ooxml/spreadsheetml/main"
	transitionalSpreadsheetMLNamespace = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
)

func (s *StrictOOXMLStrategy) SourceExt() string {
	return global.FileExtXLSX
}

func (s *StrictOOXMLStrategy) TargetExt() string {
	return global.FileExtXLSX
}

func (s *StrictOOXMLStrategy) ShouldConvert(ctx *gin.Context, fileInfo *storage.FileInfo) (bool, error) {
	if strings.ToLower(fileInfo.FileExt) != global.FileExtXLSX {
		return false, nil
	}

	return isStrictXLSX(ctx, fileInfo.StoragePath)
}

func isStrictXLSX(ctx *gin.Context, storagePath string) (bool, error) {
	storager, err := fs.GetForestStorage()
	if err != nil {
		return false, err
	}

	reader, err := storager.ReadFile(storagePath)
	if err != nil {
		logs.ErrorContextf(ctx, "isStrictXLSX ReadFile error: %v", err)
		return false, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		logs.ErrorContextf(ctx, "isStrictXLSX ReadAll error: %v", err)
		return false, err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		logs.ErrorContextf(ctx, "isStrictXLSX zip.NewReader error: %v", err)
		return false, err
	}

	for _, file := range zipReader.File {
		if file.Name == "xl/workbook.xml" {
			rc, err := file.Open()
			if err != nil {
				return false, err
			}
			defer rc.Close()

			return isStrictWorkbookXML(rc)
		}
	}

	return false, nil
}

func isStrictWorkbookXML(reader io.Reader) (bool, error) {
	decoder := xml.NewDecoder(reader)
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return false, nil
			}
			return false, err
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		switch start.Name.Space {
		case strictSpreadsheetMLNamespace:
			return true, nil
		case transitionalSpreadsheetMLNamespace:
			return false, nil
		default:
			return false, nil
		}
	}
}
