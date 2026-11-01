package forest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/insmtx/corekg/apps/kechat/models/chattype"
	"github.com/insmtx/corekg/apps/kecore/models/foresttype"
	"github.com/insmtx/corekg/apps/kecore/models/fs"
	"github.com/insmtx/corekg/apps/kecore/models/perm"
	"github.com/insmtx/corekg/apps/ketask/models/ragtypes"
	"github.com/insmtx/corekg/pkgs/task"
	"github.com/insmtx/corekg/pkgs/utils"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/insmtx/corekg/pkgs/utils/sqlutil"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/config"
	"github.com/ygpkg/yg-go/dbtools/estool"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/settings"
	"github.com/ygpkg/yg-go/types"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CheckForestNameExists 检查是否存在相同的name，并且属于同一个uin
// v2.2.0 remove uin filter, make name to be unique in global
func CheckForestNameExists(ctx context.Context, id uint, name string, companyID uint) bool {
	var existingForest foresttype.KnownowForest
	sql := dbutil.Knownow().WithContext(ctx).Table(foresttype.TableNameKnownowForest).
		Where("name = ?", name).
		Where("deleted_at is null")
	if id > 0 {
		sql = sql.Where("id <> ?", id)
	}
	if companyID > 0 {
		sql = sql.Where("company_id = ?", companyID)
	}
	err := sql.First(&existingForest).Error

	// 如果没有找到相同的name，返回false
	return err == nil
}

func CreateForest(ctx context.Context, tx *gorm.DB, forest *foresttype.KnownowForest) (uint, error) {
	var createdForest foresttype.KnownowForest
	if err := tx.Transaction(func(tx *gorm.DB) error {
		tx = tx.WithContext(ctx)
		err := tx.Table(foresttype.TableNameKnownowForest).Create(forest).Error
		if err != nil {
			return fmt.Errorf("failed to create forest: %v", err)
		}
		// 使用Create后显式获取ID
		// 显式查询刚创建的记录以确保获取ID
		if err = tx.Table(foresttype.TableNameKnownowForest).
			Where("uin = ? AND name = ?", forest.Uin, forest.Name).
			First(&createdForest).Error; err != nil {
			return fmt.Errorf("failed to get forest: %v", err)
		}

		//construct scope
		return tx.Create(&foresttype.KeResourceScope{
			ResourceType: foresttype.ResourceTypeForest,
			ResourceID:   createdForest.ID,
			ScopeType:    foresttype.ScopeTypeUser,
			ScopeID:      forest.Uin,
			Action:       foresttype.ActionManage,
		}).Error
	}); err != nil {
		return 0, err
	}

	return createdForest.ID, nil
}

type AgentRef struct {
	chattype.ChatAgent
	ForestOption *chattype.ForestChatOption `json:"forest_option"`
}

// QueryListForest 查询知识森林列表
func QueryListForest(ctx context.Context, opt apiobj.PageQuery, forestList *ForestInfoItemList) error {
	query := dbutil.Knownow().WithContext(ctx).Table(foresttype.TableNameKnownowForest+" AS forest").
		Select("forest.*,COUNT(f.id) AS file_count,SUM(f.size) AS total_size").
		Where("forest.deleted_at IS NULL").
		Group("forest.id").
		Joins("left join "+foresttype.TableNameKnownowForestFile+" f on f.forest_id = forest.id AND "+
			"f.is_dir = ? AND f.deleted_at IS NULL AND f.status = ?", -1, foresttype.FileStatusNormal)
	for _, filter := range opt.Filters {
		switch filter.Field {
		case "id":
			query = query.Where("forest.id IN (?)", filter.Value)
		case "name":
			query = query.Where("forest.name = ?", filter.Value[0])
		case "knowledge_status":
			query = query.Where("forest.knowledge_status = ?", filter.Value[0])
		case "forest_type":
			query = query.Where("forest.forest_type IN (?)", filter.Value)
		default:
			logs.WarnContextf(ctx, "[knownow-forest][QueryListForest] invalid filter field: %s", filter.Field)
			return fmt.Errorf("invalid filter field: %s", filter.Field)
		}
	}

	// ======== 核心权限控制逻辑 ========
	// 构建有权限查看的 forest_id 子查询
	authIDs := dbutil.Knownow().Table(foresttype.TableNameKeResourceScope).
		Select("resource_id").
		Where("resource_type = ? AND deleted_at IS NULL", foresttype.ResourceTypeForest).
		Where("("+
			// 1. 公开权限 (action = 'view' 且 scope_type = 'public')
			"(action = ? AND scope_type = ?) OR "+
			// 2. 个人管理权限 (action = 'manage' 且 scope_type = 'user' 且 scope_id = 当前用户)
			"(action = ? AND scope_type = ? AND scope_id = ?) OR "+
			// 3. 个人查看权限 (action = 'view' 且 scope_type = 'user' 且 scope_id = 当前用户)
			"(action = ? AND scope_type = ? AND scope_id = ?) OR "+
			// 4. 公司权限 (action = 'view' 且 scope_type = 'company' 且 scope_id = 当前公司)
			"(action = ? AND scope_type = ? AND scope_id = ?)"+
			")", foresttype.ActionView, foresttype.ScopeTypePublic, // 公开
			foresttype.ActionManage, foresttype.ScopeTypeUser, opt.Uin, // 个人管理
			foresttype.ActionView, foresttype.ScopeTypeUser, opt.Uin, // 个人查看
			foresttype.ActionView, foresttype.ScopeTypeCompany, opt.CompanyID) // 公司权限

	//有权限查看
	query = query.Where("forest.id IN (?)", authIDs)

	if err := query.Count(&forestList.Total).Error; err != nil {
		return err
	}
	if forestList.Total == 0 {
		return nil
	}

	if len(opt.OrderBy) > 0 {
		query = query.Order(strings.Join(opt.OrderBy, ","))
	}

	query = query.Offset(opt.Offset)
	if !opt.ListAll && opt.Limit > 0 {
		query = query.Limit(opt.Limit)
	}

	if err := query.Find(&forestList.Data).Error; err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][QueryListForest] failed to query data: %v", err)
		return err
	}

	agRefs := make([]*AgentRef, 0)
	if err := dbutil.Chat().Table(chattype.TableNameAgent + " AS a").
		Select("a.*,av.forest_option as forest_option").
		Where("a.deleted_at IS NULL").
		Joins("INNER JOIN " + chattype.TableNameAgentVersion + " AS av ON a.version = av.id AND av.deleted_at IS NULL").
		Scan(&agRefs).
		Error; err != nil {
		return err
	}

	frsRefMap := make(map[uint][]*AgentRef)
	for _, ref := range agRefs {
		if ref.ForestOption != nil && len(ref.ForestOption.ForestIDs) > 0 {
			for _, v := range ref.ForestOption.ForestIDs {
				aRef, ok := frsRefMap[v]
				if !ok {
					frsRefMap[v] = []*AgentRef{ref}
				} else {
					frsRefMap[v] = append(aRef, ref)
				}
			}
		}
	}

	// 查询管理权限
	manageForestIDs := make(map[uint]bool)

	manageScopesIDs := perm.GetManageList(ctx, opt.Uin, foresttype.ResourceTypeForest)
	for _, scope := range manageScopesIDs {
		manageForestIDs[scope] = true
	}

	var (
		qaFrsIDs []uint
		dbFrsIDs []uint
		//TODO fix es index with range or other way
		esIndex string

		qaCountMap = make(map[uint]int, 0)
		dbCountMap = make(map[uint]int, 0)
	)

	for _, frs := range forestList.Data {
		frs.IsAdmin = manageForestIDs[frs.ID]
		if refs, ok := frsRefMap[frs.ID]; !ok {
			frs.AppCount = 0
		} else {
			frs.AppCount = uint(len(refs))
		}
		if frs.ForestType == foresttype.ForestTypeQA {
			qaFrsIDs = append(qaFrsIDs, frs.ID)
			if len(esIndex) <= 0 {
				esIndex = frs.EsIndex()
			}
		}
		if frs.DataSourceType == foresttype.ForestDataSourceDB {
			dbFrsIDs = append(dbFrsIDs, frs.ID)
		}
	}

	if len(qaFrsIDs) > 0 || len(dbFrsIDs) > 0 {
		g, sonCtx := errgroup.WithContext(ctx)
		if len(qaFrsIDs) > 0 {
			g.Go(func() error {
				count, err := GetQuestionCountsByForests(sonCtx, esIndex, qaFrsIDs)
				if err != nil {
					logs.ErrorContextf(ctx, "[knownow-forest][QueryListForest] failed to get question counts: %v", err)
					return err
				}
				for _, v := range count {
					qaCountMap[v.ForestID] = v.Count
				}
				return nil
			})
		}
		if len(dbFrsIDs) > 0 {
			g.Go(func() error {
				count, err := GetDBTableCountByForestIDs(sonCtx, dbFrsIDs)
				if err != nil {
					logs.ErrorContextf(ctx, "[knownow-forest][QueryListForest] failed to get db table counts: %v", err)
					return err
				}
				for _, v := range count {
					dbCountMap[v.ForestID] = v.Count
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			logs.ErrorContextf(ctx, "[knownow-forest][QueryListForest] failed to wait: %v", err)
			return err
		}
		for _, v := range forestList.Data {
			if v.ForestType == foresttype.ForestTypeQA {
				v.FileCount = int64(qaCountMap[v.ID])
			}
			if v.DataSourceType == foresttype.ForestDataSourceDB {
				v.FileCount = int64(dbCountMap[v.ID])
			}
		}
	}

	return nil
}

// GetForestByID 通过ID获取知识森林
func GetForestByID(ctx context.Context, id uint) (*foresttype.KnownowForest, error) {
	forest := &foresttype.KnownowForest{}
	db := dbutil.Knownow().WithContext(ctx)
	err := db.Table(foresttype.TableNameKnownowForest).
		Where("id = ?", id).
		First(forest).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logs.WarnContextf(ctx, "[knownow-forest][GetForest] get forest by id failed: %v", err)
			return nil, err
		}
		logs.ErrorContextf(ctx, "[knownow-forest][GetForest] failed to get forest: %v", err)
		return nil, err
	}
	return forest, nil
}

// GetForestByName 通过ID获取知识森林
func GetForestByName(ctx context.Context, uin uint, name string) (*foresttype.KnownowForest, error) {
	forest := &foresttype.KnownowForest{}
	db := dbutil.Knownow()
	err := db.Table(foresttype.TableNameKnownowForest).WithContext(ctx).
		Where("name = ? AND uin = ?", name, uin).
		First(forest).Error
	if err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][GetForest] failed to get forest: %v", err)
		return nil, err
	}
	return forest, nil
}

// ModifyForest 修改知识森林
func ModifyForest(ctx context.Context, forest *foresttype.KnownowForest) error {
	db := dbutil.Knownow()

	// 执行更新操作
	err := db.Table(foresttype.TableNameKnownowForest).WithContext(ctx).
		Where("id = ? AND uin = ?", forest.ID, forest.Uin).
		Updates(map[string]interface{}{
			"name":        forest.Name,
			"avatar_url":  forest.AvatarUrl,
			"description": forest.Description,
		}).Error

	// 检查是否有错误发生
	if err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][ModifyForest] failed to modify forest: %v", err)
		return fmt.Errorf("failure to modify the knowledge forest: %v", err)
	}

	return nil
}

// DeleteForest 删除知识森林
func DeleteForest(ctx context.Context, tx *gorm.DB, uin uint, forest_id uint) error {
	// 执行删除知识森林操作
	err := tx.Table(foresttype.TableNameKnownowForest).WithContext(ctx).
		Where("id = ? ", forest_id).
		Delete(&foresttype.KnownowForest{}).Error

	// 检查是否有错误发生
	if err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][DeleteForest] failed to delete forest: %v", err)
		return fmt.Errorf("failure to delete the knowledge forest: %v", err)
	}

	// 删除知识森林文件
	err = tx.Table(foresttype.TableNameKnownowForestFile).
		Where("forest_id = ?", forest_id).
		Delete(&foresttype.KnownowForestFile{}).Error

	if err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][DeleteForest] failed to delete forest_file: %v", err)
		return fmt.Errorf("failed to delete the knowledge forest file: %v", err)
	}

	return nil
}

// QueryListForestFile 查询知识森林列表，并返回路径列表
func QueryListForestFile(ctx context.Context, uin, forestID uint) ([]string, error) {
	// 获取数据库连接
	db := dbutil.Knownow()

	// 构建查询语句
	query := db.Table(foresttype.TableNameKnownowForestFile).WithContext(ctx).
		Where(foresttype.TableNameKnownowForestFile+".`uin` = ?", uin).
		Where(foresttype.TableNameKnownowForestFile+".`forest_id` = ?", forestID).
		Where("deleted_at is null")

	// 执行查询
	var files []*foresttype.KnownowForestFile
	err := query.Find(&files).Error
	if err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][QueryListForestFile] failed to query list of forest files: %v", err)
		return nil, fmt.Errorf("failed to query list of forest files: %v", err)
	}

	// 使用缓存存储已经查询过的文件信息
	fileCache := make(map[uint]*foresttype.KnownowForestFile)
	for _, file := range files {
		fileCache[file.ID] = file
	}

	// 拼接路径
	var paths []string
	for _, file := range files {
		path, err := BuildPathWithCache(fileCache, file)
		if err != nil {
			logs.ErrorContextf(ctx, "[knownow-forest][QueryListForestFile] failed to build path for file %d: %v", file.ID, err)
			return nil, fmt.Errorf("failed to build path for file %d: %v", file.ID, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}

// BuildPathWithCache 使用缓存递归构建路径
func BuildPathWithCache(fileCache map[uint]*foresttype.KnownowForestFile, file *foresttype.KnownowForestFile) (string, error) {
	if file.ParentID == 0 {
		return file.Name, nil
	}

	// 从缓存中获取父文件
	parentFile, ok := fileCache[file.ParentID]
	if !ok {
		return "", fmt.Errorf("parent file with ID %d not found in cache", file.ParentID)
	}

	// 递归构建父路径
	parentPath, err := BuildPathWithCache(fileCache, parentFile)
	if err != nil {
		return "", fmt.Errorf("failed to build parent path: %v", err)
	}

	return parentPath + file.Name, nil
}

// CreateDir 创建目录
func CreateDir(ctx context.Context, file *foresttype.KnownowForestFile) error {
	err := dbutil.Knownow().WithContext(ctx).Table(foresttype.TableNameKnownowForestFile).Create(file).Error
	if err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][CreateDir] failed to create forest file: %v", err)
		return fmt.Errorf("failed to create the knowledge forest file: %v", err)
	}

	return nil
}

type DeleteDirOption struct {
	ForestID uint   `json:"forest_id"`
	Path     string `json:"path"`
}

// HandleMove 处理移动操作
func HandleMove(ctx context.Context, srcFile, dstParent *foresttype.KnownowForestFile) error {
	db := dbutil.Knownow()

	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][HandleMove] failed to start transaction: %v", tx.Error)
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	// 如果移动的是文件夹，批量更新文件夹内的所有文件和子目录
	if srcFile.IsDir.Value() {
		var (
			dstParentIDs string
			dstDepth     int
		)

		// 保存移动前的原始信息
		originalParentIDs := srcFile.ParentIDs // 移动前的 parent_ids，比如 "3486/"
		originalDepth := srcFile.Depth

		// 计算移动前该目录下所有子文件的 parent_ids 前缀
		oldFullParentIDs := originalParentIDs + strconv.Itoa(int(srcFile.ID)) + "/" // "3486/3487/"

		if dstParent == nil {
			dstParentIDs = ""
			dstDepth = 1
			srcFile.ParentID = 0
		} else {
			dstParentIDs = dstParent.ParentIDs + strconv.Itoa(int(dstParent.ID)) + "/" // "3486/3490/"
			dstDepth = dstParent.Depth + 1
			srcFile.ParentID = dstParent.ID
		}

		// 新的完整路径
		newFullParentIDs := dstParentIDs + strconv.Itoa(int(srcFile.ID)) + "/" // "3486/3490/3487/"

		srcFile.ParentIDs = dstParentIDs
		srcFile.Depth = dstDepth

		filesDepthDiff := dstDepth - originalDepth

		// 更新当前目录
		if err := tx.Save(srcFile).Error; err != nil {
			tx.Rollback()
			logs.ErrorContextf(ctx, "[knownow-forest][HandleMove] failed to update directory parent_id and parent_ids: %v", err)
			return fmt.Errorf("failed to update directory parent_id and parent_ids: %v", err)
		}

		// 先查询匹配的记录数
		var count int64
		tx.Model(&foresttype.KnownowForestFile{}).
			Where("forest_id = ? AND deleted_at IS NULL AND parent_ids LIKE ? AND depth > ?", srcFile.ForestID, oldFullParentIDs+"%", originalDepth).
			Count(&count)
		logs.DebugContextf(ctx, "[knownow-forest][HandleMove] Found %d children to update", count)

		// 更新子文件和子目录
		if count > 0 {
			result := tx.Exec(`
                UPDATE `+foresttype.TableNameKnownowForestFile+`
                SET 
                    parent_ids = CONCAT(?, SUBSTRING(parent_ids, ?)),
                    depth = depth + ?
                WHERE 
                    forest_id = ? 
                    AND deleted_at IS NULL 
                    AND parent_ids LIKE ? 
                    AND depth > ?
            `, newFullParentIDs, // 新前缀: "3486/3490/3487/"
				len(oldFullParentIDs)+1, // 跳过的字符数: 12 (len("3486/3487/")+1)
				filesDepthDiff,          // 深度差值
				srcFile.ForestID,        // forest_id
				oldFullParentIDs+"%",    // 旧前缀匹配: "3486/3487/%"
				originalDepth,           // 深度条件
			)

			if result.Error != nil {
				tx.Rollback()
				logs.ErrorContextf(ctx, "[knownow-forest][HandleMove] failed to update children: %v", result.Error)
				return fmt.Errorf("failed to update children: %v", result.Error)
			}

			logs.InfoContextf(ctx, "[knownow-forest][HandleMove] successfully updated %d children records", result.RowsAffected)
		}

	} else {
		// 处理单个文件的移动
		if dstParent == nil {
			srcFile.ParentID = 0
			srcFile.ParentIDs = ""
			srcFile.Depth = 1
		} else {
			srcFile.ParentID = dstParent.ID
			srcFile.ParentIDs = dstParent.ParentIDs + strconv.Itoa(int(dstParent.ID)) + "/"
			srcFile.Depth = dstParent.Depth + 1
		}
		if err := tx.Save(srcFile).Error; err != nil {
			tx.Rollback()
			logs.ErrorContextf(ctx, "[knownow-forest][HandleMove] failed to update file or directory parent_id and parent_ids: %v", err)
			return fmt.Errorf("failed to update file or directory parent_id and parent_ids: %v", err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logs.ErrorContextf(ctx, "[knownow-forest][HandleMove] failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// PreviewFile 文件预览
func PreviewFile(p string) (content []byte, err error) {
	file, err := fs.Forest.ReadFile(p)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// QueryForestFile 查询知识森林文件
func QueryForestFile(ctx context.Context, opt apiobj.PageQuery) (*QueryForestFileResponse, error) {
	queryList := &QueryForestFileResponse{}
	sql := dbutil.Knownow().Table(foresttype.TableNameKnownowForestFile+" AS ff").
		Where("ff.status = ?", foresttype.FileStatusNormal).
		Where("ff.deleted_at is null")

	var (
		fStat        task.TaskStatus = ""
		files        []*File
		tagIDs       []uint
		hasTagFilter bool
	)

	for _, filter := range opt.Filters {
		switch filter.Field {
		case "id":
			sql = sql.Where("ff.id IN ?", filter.Value)
		case "forest_id":
			sql = sql.Where("ff.forest_id = ?", filter.Value[0])
		case "parent_id":
			sql = sql.Where("ff.parent_id = ?", filter.Value[0])
		case "name":
			sql = sql.Where("ff.name LIKE ?", "%"+filter.Value[0]+"%")
		case "parse_status":
			sql = sql.Where("ff.parse_status = ?", filter.Value[0])
		case "is_dir":
			sql = sql.Where("ff.is_dir = ?", filter.Value[0])
		case "file_status":
			fStat = task.TaskStatus(filter.Value[0])
			logs.DebugContextf(ctx, "[forest][QueryForestFile] file_status filter value %v %v", filter.Value[0], fStat)
			sql = sql.Where("ff.is_dir = ?", -1)
		case "enable":
			sql = sql.Where("ff.enable = ?", filter.Value[0])
		case "tag_ids":
			// 将字符串tag ID转换为uint
			hasTagFilter = true
			for _, tagIDStr := range filter.Value {
				if tagID, err := strconv.ParseUint(tagIDStr, 10, 32); err == nil {
					tagIDs = append(tagIDs, uint(tagID))
				} else {
					logs.ErrorContextf(ctx, "QueryForestFile invalid tag_id: %s, error: %v", tagIDStr, err)
					return nil, fmt.Errorf("invalid tag_id: %s", tagIDStr)
				}
			}
		default:
			logs.ErrorContextf(ctx, "QueryForestFile invalid filtering field: %s", filter.Field)
			return nil, fmt.Errorf("invalid filtering field: %s", filter.Field)
		}
	}

	// 如果有tag筛选条件，通过JOIN ResourceTag表来筛选
	if hasTagFilter && len(tagIDs) > 0 {
		sql = sql.Distinct().
			Joins("INNER JOIN "+foresttype.TableNameResourceTag+" AS rt_filter ON rt_filter.resource_id = ff.id AND rt_filter.deleted_at IS NULL").
			Where("rt_filter.resource_type = ?", foresttype.TagResourceTypeFile).
			Where("rt_filter.tag_id IN (?)", tagIDs)

		if err := BatchSaveRecentUsedTag(ctx, dbutil.Knownow(), tagIDs, opt.Uin, opt.CompanyID); err != nil {
			logs.ErrorContextf(ctx, "BatchSaveRecentUsedTag: save tags recent used err : %v", err)
			return nil, err
		}

	}

	// ========================= filter ban list ==========================

	ap, err := NewAccessProvider(ctx, &ContextModel{
		ResourceType: foresttype.ResourceTypeForestFile,
		ScopeType:    foresttype.ScopeTypeUser,
		ScopeID:      opt.Uin,
		Action:       foresttype.ActionBan,
	}).Action(ctx)
	if err != nil {
		logs.ErrorContextf(ctx, "filter ban list failed: %v", err)
		return nil, err
	}
	if len(ap.BanList) > 0 {
		sql = sql.Where("ff.id NOT IN (?)", ap.BanList)
	}

	// ========================== filter ban list ==========================

	// 处理 BeginTime 和 EndTime
	if !opt.BeginTime.IsZero() {
		sql = sql.Where("ff.created_at >= ?", opt.BeginTime)
	}
	if !opt.EndTime.IsZero() {
		sql = sql.Where("ff.created_at <= ?", opt.EndTime)
	}
	// 如果有tag筛选，需要使用COUNT(DISTINCT)来避免JOIN导致的重复计数
	if hasTagFilter && len(tagIDs) > 0 {
		if err := sql.Select("COUNT(DISTINCT ff.id)").Count(&queryList.Response.Total).Error; err != nil {
			logs.ErrorContextf(ctx, "QueryForestFile Statistical project failed: %v", err)
			return nil, err
		}
	} else {
		if err := sql.Count(&queryList.Response.Total).Error; err != nil {
			logs.ErrorContextf(ctx, "QueryForestFile Statistical project failed: %v", err)
			return nil, err
		}
	}
	if queryList.Response.Total == 0 {
		return queryList, nil
	}
	if len(opt.OrderBy) > 0 {
		// 为OrderBy字段添加表别名前缀
		orderByFields := make([]string, 0, len(opt.OrderBy))
		for _, order := range opt.OrderBy {
			// 如果order字段不包含表别名，则添加ff.前缀
			if !strings.Contains(order, ".") {
				orderByFields = append(orderByFields, "ff."+order)
			} else {
				orderByFields = append(orderByFields, order)
			}
		}
		sql = sql.Order(strings.Join(orderByFields, ","))
	}
	sql = sql.Select("ff.*").Offset(opt.Offset)
	if !opt.ListAll && opt.Limit > 0 {
		sql = sql.Limit(opt.Limit)
	}
	if err := sql.Find(&queryList.Response.Data).Error; err != nil {
		logs.ErrorContextf(ctx, "QueryForestFile Retrieval project failed: %v", err)
		return nil, err
	}
	queryList.Response.Limit = opt.Limit
	queryList.Response.Offset = opt.Offset

	fIDs := make([]uint, 0)
	for _, v := range queryList.Response.Data {
		status, pro := v.KnownowForestFile.CalculateCompletionPercentage()

		v.FileStatus = status
		v.FileProgress = pro
		// 如果 fStat 有值，并且当前状态不匹配 fStat，则跳过当前元素。
		// 只有在 fStat 为空 (不需要过滤) 或状态匹配时，代码才会继续执行到下一行。
		if len(fStat) > 0 && task.TaskStatus(status) != fStat {
			continue // 不满足过滤条件，跳过
		}
		fIDs = append(fIDs, v.KnownowForestFile.ID)
		// 满足条件 (或没有过滤条件) 的元素才会被添加
		files = append(files, v)
	}

	var tags []ResourceTag
	if err := dbutil.Knownow().Table(foresttype.TableNameResourceTag+" AS rt").
		Where("rt.deleted_at IS NULL").
		Where("rt.resource_id IN (?)", fIDs).
		Where("rt.resource_type = ?", foresttype.TagResourceTypeFile).
		Where("tg.status = ?", foresttype.TagGroupStatusEnable).
		Joins("LEFT JOIN " + foresttype.TableNameTag + " AS t ON rt.tag_id = t.id AND t.deleted_at IS NULL").
		Joins("LEFT JOIN " + foresttype.TableNameTagGroup + " AS tg ON t.group_id = tg.id AND tg.deleted_at IS NULL").
		Select("rt.id, rt.resource_id, rt.resource_type, rt.tag_id, t.name as tag_name, tg.name as tag_group_name").
		Find(&tags).
		Error; err != nil {
		logs.ErrorContextf(ctx, "QueryForestFile failed to get tags: %v", err)
		return nil, err
	}

	fTagMap := make(map[uint][]ResourceTag)
	for _, tag := range tags {
		fTagMap[tag.ResourceID] = append(fTagMap[tag.ResourceID], tag)
	}

	for _, v := range files {
		v.TagList = fTagMap[v.KnownowForestFile.ID]
	}

	queryList.Response.Data = files
	return queryList, nil
}

type ResourceTag struct {
	ID           uint                       `json:"id"`
	ResourceID   uint                       `json:"resource_id"`
	ResourceType foresttype.TagResourceType `json:"resource_type"`
	TagID        uint                       `json:"tag_id"`
	TagName      string                     `json:"tag_name"`
	TagGroupName string                     `json:"tag_group_name"`
}

// GetForestByIDS 根据ids获取知识森林
func GetForestByIDS(ids []uint) ([]*foresttype.KnownowForest, error) {
	var forests []*foresttype.KnownowForest
	if err := dbutil.Knownow().Where("id in (?)", ids).Find(&forests).Error; err != nil {
		return nil, err
	}
	return forests, nil
}

// QueryForestFileResponse 获取试卷列表
type QueryForestFileResponse struct {
	apiobj.BaseResponse
	Response struct {
		apiobj.QueryResponse
		Data []*File `json:"data"`
	}
}

type File struct {
	foresttype.KnownowForestFile
	FileStatus        string `json:"file_status"`
	FileProgress      string `json:"file_progress"`
	ForestType        string `json:"forest_type"`
	DataSourceType    string `json:"data_source_type"`
	DataSourceSubType string `gorm:"column:data_source_subtype" json:"data_source_subtype"`
	// TagList 标签列表
	TagList []ResourceTag `gorm:"-" json:"tag_list"`
}

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)

// FormatFileSize 将字节大小转换为用量字符串
// size: 文件大小（字节）
// 返回格式化的用量字符串，如 "2.4G"、"128.5MB"、"1.2KB"、"512B"
func FormatFileSize(size int64) string {

	switch {
	case size >= GB:
		// 转换为GB，保留1位小数
		gb := float64(size) / float64(GB)
		return fmt.Sprintf("%.1fG", gb)
	case size >= MB:
		// 转换为MB，保留1位小数
		mb := float64(size) / float64(MB)
		return fmt.Sprintf("%.1fMB", mb)
	case size >= KB:
		// 转换为KB，保留1位小数
		kb := float64(size) / float64(KB)
		return fmt.Sprintf("%.1fKB", kb)
	default:
		// 小于1KB直接显示字节数
		return fmt.Sprintf("%dB", size)
	}
}

// RefreshForests will refresh all input ids related forest's aggregation value
// now it will update count, disk_storage
func RefreshForests(ctx2 context.Context, ids []uint) error {
	if len(ids) == 0 {
		logs.WarnContextf(ctx2, "RefreshForest called with empty IDs, nothing to do.")
		return nil
	}

	group, ctx := errgroup.WithContext(ctx2)
	semaphore := make(chan struct{}, 5)

	for _, id := range ids {
		forestID := id
		group.Go(func() error {
			select {
			case <-ctx.Done():
				logs.WarnContextf(ctx, "RefreshForest for ForestIDs %d cancelled: %v", forestID, ctx.Err())
				return ctx.Err()
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				logs.InfoContextf(ctx, "Starting refresh for ForestIDs: %d", forestID)
				if err := RefreshForest(ctx, forestID); err != nil {
					logs.ErrorContextf(ctx, "Failed to refresh aggregation for ForestIDs %d: %v", forestID, err)
					return err
				}
				logs.InfoContextf(ctx, "Successfully refreshed ForestIDs: %d", forestID)
				return nil
			}
		})
	}

	if err := group.Wait(); err != nil {
		logs.ErrorContextf(ctx2, "RefreshForest completed with errors: %v", err)
		return err
	}

	logs.InfoContextf(ctx2, "RefreshForest completed successfully for IDs: %v", ids)
	return nil
}

// RefreshForest 负责在事务中更新单个知识库的聚合值
func RefreshForest(ctx context.Context, forestID uint) error {
	return dbutil.Knownow().Transaction(func(tx *gorm.DB) error {
		var forest foresttype.KnownowForest
		if err := tx.Take(&forest, forestID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logs.WarnContextf(ctx, "Forest with ID %d not found, skipping update.", forestID)
				return nil
			} else {
				return fmt.Errorf("failed to find forest with ID %d for locking: %w", forestID, err)
			}
		}

		var files []foresttype.KnownowForestFile
		if err := tx.Find(&files, "forest_id = ?", forestID).Error; err != nil {
			return fmt.Errorf("failed to query files for ForestIDs %d within transaction: %w", forestID, err)
		}

		newCount := len(files)
		var newDiskStorage int64
		for _, file := range files {
			newDiskStorage += file.Size
		}

		updateData := map[string]interface{}{
			"count":        newCount,
			"disk_storage": FormatFileSize(newDiskStorage),
			"updated_at":   time.Now(),
		}

		updateResult := tx.Model(&forest).Updates(updateData)
		if updateResult.Error != nil {
			return fmt.Errorf("failed to update forest aggregation for ForestIDs %d within transaction: %w", forestID, updateResult.Error)
		}
		if updateResult.RowsAffected == 0 {
			logs.WarnContextf(ctx, "No rows affected when updating ForestIDs %d in transaction. Maybe no changes or record disappeared.", forestID)
		}

		return nil
	})
}

type WithPermItem struct {
	ID          uint                   `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	PublicScope foresttype.PublicScope `json:"public_scope"`
	ManagerIDs  types.UintArray        `json:"manager_ids"`
	ScopeIDs    types.UintArray        `json:"scope_ids"`
}

var ErrNameAlreadyExists = errors.New("name already exists")

// UpdateForestWithPerm 更新知识库的基本信息和权限范围
func UpdateForestWithPerm(ctx context.Context, tx *gorm.DB, opt *WithPermItem, forest *foresttype.KnownowForest) error {
	return tx.Transaction(func(tx *gorm.DB) error {
		var (
			infoChanged = false
		)

		// 检查名称是否修改
		if forest.Name != opt.Name ||
			forest.Description != opt.Description ||
			forest.PublicScope != opt.PublicScope {
			infoChanged = true
		}

		if infoChanged {
			if exists := CheckForestNameExists(ctx, opt.ID, opt.Name, forest.CompanyID); exists {
				return ErrNameAlreadyExists
			}

			forest.Name = opt.Name
			forest.Description = opt.Description
			forest.PublicScope = opt.PublicScope
			if err := tx.Save(&forest).Error; err != nil {
				return err
			}
		}

		return perm.UpdateResourceScope(ctx, tx, forest.ID, foresttype.ResourceTypeForest, opt.ScopeIDs.Slice(), opt.ManagerIDs.Slice(), opt.PublicScope, forest.CompanyID)
	})
}

func ViewAbleForests(uin, companyID uint) (res []uint, err error) {
	query := dbutil.Knownow().Table(foresttype.TableNameKnownowForest+" AS forest").
		Where("forest.deleted_at IS NULL").
		Group("forest.id").
		Joins("left join "+foresttype.TableNameKnownowForestFile+" f on f.forest_id = forest.id AND "+
			"f.is_dir = ? AND f.deleted_at IS NULL AND f.status = ?", -1, foresttype.FileStatusNormal)

	// ======== 核心权限控制逻辑 ========
	// 构建有权限查看的 forest_id 子查询
	authIDs := dbutil.Knownow().Table(foresttype.TableNameKeResourceScope).
		Select("resource_id").
		Where("resource_type = ? AND deleted_at IS NULL", foresttype.ResourceTypeForest).
		Where("("+
			// 1. 公开权限 (action = 'view' 且 scope_type = 'public')
			"(action = ? AND scope_type = ?) OR "+
			// 2. 个人管理权限 (action = 'manage' 且 scope_type = 'user' 且 scope_id = 当前用户)
			"(action = ? AND scope_type = ? AND scope_id = ?) OR "+
			// 3. 个人查看权限 (action = 'view' 且 scope_type = 'user' 且 scope_id = 当前用户)
			"(action = ? AND scope_type = ? AND scope_id = ?) OR "+
			// 4. 公司权限 (action = 'view' 且 scope_type = 'company' 且 scope_id = 当前公司)
			"(action = ? AND scope_type = ? AND scope_id = ?)"+
			")", foresttype.ActionView, foresttype.ScopeTypePublic, // 公开
			foresttype.ActionManage, foresttype.ScopeTypeUser, uin, // 个人管理
			foresttype.ActionView, foresttype.ScopeTypeUser, uin, // 个人查看
			foresttype.ActionView, foresttype.ScopeTypeCompany, companyID) // 公司权限

	//有权限查看
	if err = query.Where("forest.id IN (?)", authIDs).Pluck("forest.id", &res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func CanUseForest(ctx context.Context, frsID, uin, companyID uint) bool {
	var c int64
	if err := dbutil.Knownow().Table(foresttype.TableNameKnownowForest+" f ").
		Where("f.deleted_at IS NULL").
		Where("f.id = ?", frsID).
		Joins("LEFT JOIN "+foresttype.TableNameKnownowForestPublicScope+" ps ON f.id = ps.forest_id AND ps.deleted_at IS NULL").
		Where("("+
			"("+sqlutil.ContainsCSV("f.manager_ids")+") OR"+
			"(f.public_scope = ? AND f.company_id = ?) OR"+
			"(f.public_scope = ? AND f.uin = ?) OR"+
			"(f.public_scope = ? AND ps.scope_type = ? AND ps.scope_id = ?) OR"+
			"(f.public_scope = ? AND ps.scope_type = ? AND ps.scope_id = ?)"+
			")", uin, foresttype.PublicScopeCompany, companyID, foresttype.PublicScopePrivate, uin, foresttype.PublicScopeCustom, foresttype.ScopeTypeUser, uin, foresttype.PublicScopePublic, foresttype.ScopeTypeCompany, companyID).
		Count(&c).Error; err != nil {
		logs.ErrorContextf(ctx, "check forest use perm faild: %v", err)
		return false
	}
	return c > 0
}

func GetNameByModuleIDs(ctx context.Context, moduleIDs map[foresttype.ForestModule][]uint) (*GetNameByIDsRes, error) {
	if len(moduleIDs) == 0 {
		return &GetNameByIDsRes{
			NameList: make([]GetNameByIDsNameListItem, 0),
		}, nil
	}

	nameList := make([]GetNameByIDsNameListItem, 0)

	for module, ids := range moduleIDs {
		if len(ids) == 0 {
			continue
		}
		tableName, ok := foresttype.ForestModuleTableMap[module]
		if !ok {
			continue
		}

		type NameEntity struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		}
		var nameEntityList []NameEntity
		nameField := "name"
		if specifiedNameField, ok := foresttype.ForestModuleNameFieldMap[module]; ok {
			nameField = specifiedNameField
		}
		if err := dbutil.Knownow().WithContext(ctx).Table(tableName).Select(fmt.Sprintf("id, %s as name", nameField)).Where("id in (?)", utils.SliceDuplicate(ids)).Find(&nameEntityList).Error; err != nil {
			return nil, err
		}

		for _, nameEntity := range nameEntityList {
			nameList = append(nameList, GetNameByIDsNameListItem{
				Module: module,
				ID:     nameEntity.ID,
				Name:   nameEntity.Name,
			})
		}

	}
	return &GetNameByIDsRes{
		NameList: nameList,
	}, nil

}

// ItemCount 用于解析聚合结果的结构体
type ItemCount struct {
	ForestID uint `json:"key"`
	Count    int  `json:"doc_count"`
}

// AggregationResult 封装了聚合查询的全部响应结构
type AggregationResult struct {
	Aggregations struct {
		QuestionCountsByForest struct {
			Buckets []ItemCount `json:"buckets"`
		} `json:"question_counts_by_forest"`
	} `json:"aggregations"`
}

// GetQuestionCountsByForests 查询指定 forest_id 列表下，type="FQA" 且 qa_main_id 为空的文档总数（按 forest_id 分组）
func GetQuestionCountsByForests(ctx context.Context, index string, forestIDs []uint) ([]ItemCount, error) {

	// 1. 构建 Query DSL (使用 map[string]interface{})
	searchBody := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				// 筛选条件 (AND 关系)
				"filter": []map[string]interface{}{
					// forest_id in (?)
					{
						"terms": map[string]interface{}{
							"forest_id": forestIDs,
						},
					}, // type="FQA"
					{
						"term": map[string]interface{}{
							"type": ragtypes.ChunkTypeFQA,
						},
					},
				}, // 排除条件 (qa_main_id 为空)
				"must_not": []map[string]interface{}{
					{
						"exists": map[string]interface{}{
							"field": "qa_main_id", // 排除 qa_main_id 字段存在的文档
						},
					},
				},
			},
		}, // 2. 构建 Aggregation DSL
		"aggs": map[string]interface{}{
			"question_counts_by_forest": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "forest_id",
					"size":  len(forestIDs) + 100,
				},
			},
		},
	}

	// 3. 将 DSL 转换为 []byte
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(searchBody); err != nil {
		return nil, fmt.Errorf("failed to encode query DSL: %w", err)
	}

	logs.DebugContextf(ctx, "ES Aggregation Query DSL: %s", buf.String())

	client, err := InitESClient(ctx)
	if err != nil {
		logs.ErrorContextf(ctx, "failed to init ES client: %v", err)
		return nil, fmt.Errorf("failed to init ES client: %w", err)
	}

	// 4. 执行 ES Search 查询
	resp, err := client.Search(client.Search.WithIndex(index), client.Search.WithBody(&buf), client.Search.WithContext(context.Background()))
	if err != nil {
		logs.ErrorContextf(ctx, "failed to search index %s: %v", index, err)
		return nil, fmt.Errorf("es search request failed: %w", err)
	}
	defer resp.Body.Close()

	// 5. 处理响应结果
	if resp.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
			logs.ErrorContextf(ctx, "failed to decode search response: %v", err)
			return nil, fmt.Errorf("es error response unmarshal failed: %s", resp.Status())
		}
		return nil, fmt.Errorf("es query failed [%s]: %v", resp.Status(), e)
	}

	// 6. 反序列化聚合结果
	var result AggregationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logs.ErrorContextf(ctx, "failed to decode search response: %v", err)
		return nil, fmt.Errorf("failed to unmarshal es response: %w", err)
	}

	// 7. 返回最终的计数列表
	return result.Aggregations.QuestionCountsByForest.Buckets, nil
}

func GetDBTableCountByForestIDs(ctx context.Context, forestIDs []uint) ([]ItemCount, error) {
	if len(forestIDs) == 0 {
		logs.WarnContextf(ctx, "forestIDs is empty")
		return nil, nil
	}
	var itemCountList []ItemCount
	if err := dbutil.Knownow().WithContext(ctx).Table(foresttype.TableNameKeForestTable).
		Where("deleted_at IS NULL").
		Where("forest_id IN (?)", forestIDs).
		Select("forest_id, count(*) as count").
		Group("forest_id").
		Find(&itemCountList).Error; err != nil {
		return nil, err
	}
	return itemCountList, nil
}

func InitESClient(ctx context.Context) (*elasticsearch.Client, error) {
	// cfg := config.ESConfig{
	// 	Addresses:     []string{"http://example.com:53082/"},
	// 	SlowThreshold: time.Millisecond,
	// 	Username:      "elastic",
	// 	Password:      "CHANGE_ME_PASSWORD",
	// }
	cfg := config.ESConfig{}
	err := settings.GetYaml("knowledge", "es", &cfg)
	if err != nil {
		logs.ErrorContextf(ctx, "get es config failed: %s", err)
		return nil, err
	}
	client, err := estool.InitES(cfg)
	if err != nil {
		logs.ErrorContextf(ctx, "init es client failed: %s", err)
		return nil, err
	}
	return client, nil
}

// BatchSaveRecentUsedTag 批量保存 RecentUsedTag，更新使用次数与上次使用时间
// tagIDs: 标签ID列表
// uin: 用户ID
// companyID: 公司ID
func BatchSaveRecentUsedTag(ctx context.Context, tx *gorm.DB, tagIDs []uint, uin uint, companyID uint) error {
	if len(tagIDs) == 0 {
		return nil
	}

	tagEntityList, err := NewTagDao().GetListByCond(ctx, &TagCond{
		IDs: tagIDs,
	})
	if err != nil {
		logs.ErrorContextf(ctx, "failed to fetch tag info: %w", err)
		return err
	}
	tagMap := tagEntityList.ToMap()

	ts, err := NewRecentUsedTagDao().GetListByCond(ctx, &RecentUsedTagCond{
		TagIDs: tagIDs,
		BaseCond: BaseCond{
			Uin:       uin,
			CompanyID: companyID,
		},
	})
	if err != nil {
		logs.ErrorContextf(ctx, "failed to fetch recent used tag info: %w", err)
		return err
	}
	tUsedMap := make(map[uint]*foresttype.RecentUsedTag)
	for i := range ts {
		tUsedMap[ts[i].TagID] = &ts[i]
	}

	var toSaveUsedTag []*foresttype.RecentUsedTag
	for _, tagID := range tagIDs {
		if tag, ok := tagMap[tagID]; ok {
			if t, exists := tUsedMap[tagID]; !exists {
				// 不存在，创建新记录
				toSaveUsedTag = append(toSaveUsedTag, &foresttype.RecentUsedTag{
					CompanyID:  companyID,
					GroupID:    tag.GroupID,
					Uin:        uin,
					TagID:      tagID,
					LastUsedAt: time.Now(),
					UsageCount: 1,
				})
			} else {
				// 存在，更新使用次数和最后使用时间
				t.UsageCount++
				t.LastUsedAt = time.Now()
				toSaveUsedTag = append(toSaveUsedTag, t)
			}
		}
	}

	// 批量保存
	if len(toSaveUsedTag) > 0 {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "uin"}, {Name: "tag_id"}, {Name: "group_id"}, {Name: "company_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"usage_count", "last_used_at"}),
		}).Create(&toSaveUsedTag).Error; err != nil {
			logs.ErrorContextf(ctx, "failed to save recent used tag: %v", err)
			return err
		}
	}

	return nil
}
