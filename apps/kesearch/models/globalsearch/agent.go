package globalsearch

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/insmtx/corekg/apps/account/models/user"
	"github.com/insmtx/corekg/apps/kechat/models/chattype"
	"github.com/insmtx/corekg/apps/kecore/models/foresttype"
	"github.com/insmtx/corekg/apps/kesearch/models/essearch"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/insmtx/corekg/pkgs/utils/sqlutil"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/settings"
)

type HighLightConfig struct {
	HighLightPrefix string `yaml:"highlight_prefix"`
	HighLightSuffix string `yaml:"highlight_suffix"`
}

func defaultHighLightConfig() *HighLightConfig {
	return &HighLightConfig{
		HighLightPrefix: "<span style=\"color: #3D7FFF; font-weight: 500;\">",
		HighLightSuffix: "</span>",
	}
}

var highlightConfig = defaultHighLightConfig()

func InitHighLightConfig(ctx context.Context) error {
	cfg := defaultHighLightConfig()
	err := settings.GetYaml("knowledge", "highlight", cfg)
	if err != nil {
		logs.ErrorContextf(ctx, "get es config failed: %s", err)
		return err
	}
	highlightConfig = cfg
	return nil
}

// TODO :加入配置
// const (
// 	// 高亮前缀
// 	HighLightPrefix = "<span style=\"color: #3D7FFF; font-weight: 500;\">"
// 	// 高亮后缀
// 	HighLightSuffix = "</span>"
// )

// SearchAgentType 应用高亮
type SearchAgentType struct {
	ID          uint      `json:"id"`
	Uin         uint      `json:"uin"`
	UserName    string    `json:"user_name"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description"`
	AgentName   string    `json:"agent_name"`
	ImageURL    string    `json:"image_url,omitempty"`
	AgentType   string    `json:"agent_type"`
	// 高亮字段
	HighlightedAgentName   string `json:"highlighted_agent_name,omitempty"`
	HighlightedDescription string `json:"highlighted_description,omitempty"`
}

// SearchAgent 搜索应用
func (wrapper GlobalSearchWrapper) SearchAgent() ([]*SearchAgentType, error) {

	if len(wrapper.ForestIDs) == 0 {
		return []*SearchAgentType{}, nil
	}

	agents, err := wrapper.FindAgentsWithKeywords()
	if err != nil {
		logs.ErrorContextf(wrapper.Ctx, "FindAgentsWithKeywords error: %v", err)
		return nil, err
	}

	for i, v := range agents {
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
		agents[i] = v
	}

	ProcessHighlight(agents, wrapper.keywords)
	// for _, agent := range agents {
	// 	logs.Infof("agent: %+v", agent)
	// }
	return agents, nil
}

// FindAgentsWithKeywords 根据关键词搜索应用
func (wrapper GlobalSearchWrapper) FindAgentsWithKeywords() ([]*SearchAgentType, error) {
	var res []*SearchAgentType
	token_str := joinTokensWithBuffer(wrapper.keywords)
	// 初始化查询
	query := dbutil.Chat().Table(chattype.TableNameAgent+" a ").
		Unscoped().
		Select("a.id, "+
			"a.uin, "+
			"a.created_at, "+
			"v.description, "+
			"a.show_name as agent_name, "+
			"a.avatar_url as image_url, "+
			"v.agent_type").
		Joins("LEFT JOIN chat_agent_version v ON a.version = v.id").
		Where("a.deleted_at IS NULL").
		Where(sqlutil.RegexpInsensitive("a.show_name")+" OR "+sqlutil.RegexpInsensitive("v.description"), token_str, token_str)

	// ======== 核心权限控制逻辑 ========
	// 构建有权限查看的 agent_id 子查询
	var authIDs []uint
	if err := dbutil.Knownow().Table(foresttype.TableNameKeResourceScope).
		Where("resource_type = ? AND deleted_at IS NULL", foresttype.ResourceTypeAgent).
		Where("("+
			// 1. 公开权限 (action = 'view' 且 scope_type = 'public')
			"(action = ? AND scope_type = ?) OR "+
			// 2. 个人管理权限 (action = 'manage' 且 scope_type = 'user' 且 scope_id = 当前用户)
			"(action = ? AND scope_type = ? AND scope_id = ?) OR "+
			// 3. 个人查看权限 (action = 'view' 且 scope_type = 'user' 且 scope_id = 当前用户)
			"(action = ? AND scope_type = ? AND scope_id = ?) OR "+
			// 4. 公司权限 (action = 'view' 且 scope_type = 'company' 且 scope_id = 当前公司)
			"(action = ? AND scope_type = ? AND scope_id = ?)"+
			")",
			foresttype.ActionView, foresttype.ScopeTypePublic, // 公开
			foresttype.ActionManage, foresttype.ScopeTypeUser, wrapper.Uin, // 个人管理
			foresttype.ActionView, foresttype.ScopeTypeUser, wrapper.Uin, // 个人查看
			foresttype.ActionView, foresttype.ScopeTypeCompany, wrapper.CompanyID). // 公司权限
		Pluck("distinct resource_id", &authIDs).
		Error; err != nil {
		logs.ErrorContextf(wrapper.Ctx, "FindAgentsWithKeywords error: %v", err)
		return nil, err
	}

	otherAuth := dbutil.Chat().Table(chattype.TableNameAgent+" a ").
		Select("a.id").
		Where("a.deleted_at IS NULL").
		Where("a.publish_status = ? AND a.uin = ?", chattype.StatusDraft, wrapper.Uin)

	query = query.Where("("+
		//有权限查看
		"a.id IN (?) OR "+
		//草稿
		"a.id IN (?)"+
		")",
		authIDs,
		otherAuth)

	// 查询结果
	err := query.Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

// joinTokensWithBuffer 拼接正则关键词
func joinTokensWithBuffer(results *essearch.AnalyzeResultList) string {
	var b bytes.Buffer
	for i, res := range results.Tokens {
		if i > 0 {
			b.WriteString("|")
		}
		b.WriteString(regexp.QuoteMeta(res.Token))
	}
	return b.String()
}

// ProcessHighlight 处理整个 Agent 列表，添加高亮字段
func ProcessHighlight(agents []*SearchAgentType, keywords *essearch.AnalyzeResultList) {
	for _, agent := range agents {
		agent.HighlightedAgentName = HighlightKeywords(agent.AgentName, keywords)
		agent.HighlightedDescription = HighlightKeywords(agent.Description, keywords)
	}
}

// HighlightKeywords 高亮关键词
func HighlightKeywords(text string, keywords *essearch.AnalyzeResultList) string {
	if text == "" || len(keywords.Tokens) == 0 {
		return text
	}

	pattern := joinTokensWithBuffer(keywords)

	// 编译正则
	regex, err := regexp.Compile("(?i)(" + pattern + ")")
	if err != nil {
		return text // 出错就原样返回
	}

	// 替换匹配项为高亮形式
	highlighted := regex.ReplaceAllStringFunc(text, func(match string) string {
		return fmt.Sprintf("%s%s%s", highlightConfig.HighLightPrefix, match, highlightConfig.HighLightSuffix)
	})

	return highlighted
}
