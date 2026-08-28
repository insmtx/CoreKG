package fs

import (
	"context"
	"fmt"

	"github.com/ygpkg/yg-go/config"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/settings"
	"github.com/ygpkg/yg-go/storage"
)

const (
	PurposeKeFile     = "ke"       // 知识森林文件根路径
	PurposeForestAlgo = "algo-lke" // 知识森林算法根路径
	PurposeMdImage    = "md-img"   // markdown图片路径
)

var (
	Forest storage.Storager
	Cfg    config.StorageConfig
)

// InitForestStorage 新建Storager对象
func InitForestStorage() error {
	ctx := context.TODO()
	stForest, err := storage.LoadStorager(PurposeKeFile)
	if err != nil {
		return err
	}
	Forest = stForest
	if err = settings.GetYaml("core", storage.SettingPrefix+PurposeKeFile, &Cfg); err != nil {
		logs.ErrorContextf(ctx, "Get corekg bucket cfg err: %v", err)
		return err
	}

	return nil
}

// GetForestStorage returns the internal knowledge-base file storage.
func GetForestStorage() (storage.Storager, error) {
	if Forest == nil {
		if err := InitForestStorage(); err != nil {
			return nil, err
		}
	}
	if Forest == nil {
		return nil, fmt.Errorf("forest storage is not initialized")
	}
	return Forest, nil
}
