package svcforestfile

import (
	"context"
	"strings"

	"github.com/insmtx/corekg/apps/kecore/models/fs"
	"github.com/insmtx/corekg/pkgs/utils/s3util"
	"github.com/ygpkg/yg-go/config"
	"github.com/ygpkg/yg-go/settings"
	"github.com/ygpkg/yg-go/storage"
	"gopkg.in/yaml.v3"
)

type presignedURLGenerator interface {
	GeneratePresignedURL(context.Context, *storage.GeneratePresignedURLInput) (*string, error)
}

type publicStorageConfig struct {
	S3 struct {
		PublicEndPoint string `yaml:"public_end_point"`
	} `yaml:"s3"`
}

func getInternalForestStorage() (storage.Storager, error) {
	return fs.GetForestStorage()
}

func getPublicForestPresigner() (presignedURLGenerator, error) {
	internal, err := getInternalForestStorage()
	if err != nil {
		return nil, err
	}

	value, err := settings.GetValue(settings.SettingGroupCore, storage.SettingPrefix+fs.PurposeKeFile)
	if err != nil {
		return nil, err
	}

	var cfg config.StorageConfig
	if err := yaml.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, err
	}
	if cfg.S3 == nil {
		return internal, nil
	}

	var publicCfg publicStorageConfig
	if err := yaml.Unmarshal([]byte(value), &publicCfg); err != nil {
		return nil, err
	}
	publicEndpoint := strings.TrimSpace(publicCfg.S3.PublicEndPoint)
	if publicEndpoint == "" {
		return internal, nil
	}

	return s3util.NewPresigner(*cfg.S3, cfg.StorageOption, publicEndpoint)
}
