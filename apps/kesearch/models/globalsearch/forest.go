package globalsearch

import (
	"time"

	"github.com/insmtx/corekg/apps/account/models/user"
	"github.com/insmtx/corekg/apps/kecore/models/foresttype"
	"github.com/insmtx/corekg/apps/kesearch/models/essearch"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/insmtx/corekg/pkgs/utils/sqlutil"
	"github.com/ygpkg/yg-go/logs"
)

// SearchForestType 知识库高亮
type SearchForestType struct {
	ID                uint      `json:"id"`
	Uin               uint      `json:"uin"`
	Description       string    `json:"description"`
	ForestName        string    `json:"forest_name"`
	UserName          string    `json:"user_name"`
	AvatarURL         string    `json:"avatar_url"`
	CreatedAt         time.Time `json:"created_at"`
	ImageURL          string    `json:"image_url,omitempty"`
	ForestType        string    `json:"forest_type"`
	DataSourceType    string    `json:"data_source_type"`
	DataSourceSubType string    `json:"data_source_subtype"`
	// 高亮字段
	HighlightedForestName  string `json:"highlighted_forest_name,omitempty"`
	HighlightedDescription string `json:"highlighted_description,omitempty"`
}

// SearchForest 搜索知识库
func (wrapper GlobalSearchWrapper) SearchForest() ([]*SearchForestType, error) {
	forests, err := wrapper.FindForestWithKeywords()
	if err != nil {
		logs.ErrorContext(wrapper.Ctx, err)
		return nil, err
	}

	for i, v := range forests {
		userEntity, exists := wrapper.userMap[v.Uin]
		if !exists {
			userEntity, err = user.GetUserByUin(wrapper.Ctx, v.Uin)
			if err != nil {
				logs.ErrorContextf(wrapper.Ctx, "GetUserByUin error: %v", err)
				continue
			}
			wrapper.userMap[v.Uin] = userEntity
		}
		v.UserName = userEntity.Name
		v.AvatarURL = userEntity.AvatarURL

		forests[i] = v
	}

	ProcessForestHighlight(forests, wrapper.keywords)
	return forests, nil
}

func (wrapper GlobalSearchWrapper) FindForestWithKeywords() ([]*SearchForestType, error) {
	var res []*SearchForestType
	token_str := joinTokensWithBuffer(wrapper.keywords)
	customScopeSubQuery := dbutil.Knownow().Table(foresttype.TableNameKnownowForestPublicScope+" AS ps").
		Select("ps.forest_id").
		Where("ps.scope_type = ? AND ps.scope_id = ?", foresttype.ScopeTypeUser, wrapper.Uin)

	query := dbutil.Knownow().Table(foresttype.TableNameKnownowForest+" AS forest").
		Select("forest.id AS id, "+
			"forest.name AS forest_name, "+
			"forest.description AS description, "+
			"forest.forest_type, "+
			"forest.uin, "+
			"forest.created_at, "+
			"forest.data_source_type, "+
			"forest.data_source_subtype, "+
			"forest.avatar_url AS image_url ").
		Where("forest.deleted_at IS NULL").
		Where(sqlutil.RegexpInsensitive("forest.name")+" OR "+sqlutil.RegexpInsensitive("forest.description"), token_str, token_str)
	// Where("f.is_dir = ?", -1).
	// Where("f.deleted_at IS NULL")

	if len(wrapper.ForestIDs) > 0 {
		query = query.Where("forest.id IN (?)", wrapper.ForestIDs)
	}

	query = query.Where("("+
		// 创建者或管理员
		"(forest.uin = ? AND "+sqlutil.ContainsCSV("forest.manager_ids")+") OR "+
		// 管理员身份
		sqlutil.ContainsCSV("forest.manager_ids")+" OR "+
		// 公司公开
		"(forest.public_scope = ? AND forest.company_id = ?) OR "+
		// 自定义公开
		"(forest.public_scope = ? AND forest.id IN (?)) OR "+
		// 私有自己创建
		"(forest.public_scope = ? AND forest.uin = ?)"+
		")",
		wrapper.Uin, wrapper.Uin,
		wrapper.Uin,
		foresttype.PublicScopeCompany, wrapper.CompanyID,
		foresttype.PublicScopeCustom, customScopeSubQuery,
		foresttype.PublicScopePrivate, wrapper.Uin,
	)
	if wrapper.SubjectCount > 0 {
		query = query.Limit(wrapper.SubjectCount)
	}
	err := query.Find(&res).Error
	if err != nil {
		logs.ErrorContextf(wrapper.Ctx, "[knownow-forest][QueryListForest] failed to query data: %v", err)
		return nil, err
	}

	return res, nil
}

// ProcessHighlight 处理整个 Agent 列表，添加高亮字段
func ProcessForestHighlight(forests []*SearchForestType, keywords *essearch.AnalyzeResultList) {
	for _, agent := range forests {
		agent.HighlightedForestName = HighlightKeywords(agent.ForestName, keywords)
		agent.HighlightedDescription = HighlightKeywords(agent.Description, keywords)
	}
}
