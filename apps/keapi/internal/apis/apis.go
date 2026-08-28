package apis

import (
	"github.com/insmtx/corekg/apps/account/accountmds"
	keapiforestctl "github.com/insmtx/corekg/apps/keapi/internal/apis/forestctl"
	keapisearchctl "github.com/insmtx/corekg/apps/keapi/internal/apis/searchctl"
	keapimiddleware "github.com/insmtx/corekg/apps/keapi/internal/middleware"
	"github.com/ygpkg/yg-go/apis/runtime/server"
	"github.com/ygpkg/yg-go/metrics"
)

// RegistryRouter 注册路由
func RegistryRouter(eng *server.Router) {
	eng.HandleDoc("keapi")
	eng.G("keapi.metrics", metrics.GetHandler())
	eng.AuthInject("", accountmds.InjectLoginStatus)

	// 对外知识库接口
	eng.P("keapi.ListForest", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.ListForest)
	eng.P("keapi.BatchGetForest", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.BatchGetForest)
	eng.P("keapi.CreateForest", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.CreateForest)
	eng.P("keapi.UpdateForest", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.UpdateForest)
	eng.P("keapi.DeleteForest", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.DeleteForest)

	// 对外文档接口
	eng.P("keapi.ListFile", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.ListFile)
	eng.P("keapi.BatchGetFile", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.BatchGetFile)
	eng.P("keapi.GetFileChunks", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.GetFileChunksBySequences)
	eng.P("keapi.UploadFile", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.UploadFile)
	eng.P("keapi.PreviewFileByURL", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.PreviewFileByURL)
	// node
	eng.P("keapi.CreateDir", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.CreateDir)
	eng.P("keapi.RenamePath", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.RenamePath)
	eng.P("keapi.DeletePath", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.DeletePath)
	// chat
	eng.P("keapi.CreateChat", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.CreateChatSession)
	eng.P("keapi.BatchGetChatInfo", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.BatchGetChatSession)
	eng.P("keapi.UpdateChatName", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.UpdateChatName)
	eng.P("keapi.DeleteChat", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.DeleteChatSession)
	eng.P("keapi.CreateChatMessage", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.CreateChatMessage)
	eng.P("keapi.ListChatMessages", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.ListChatSessionMessages)
	eng.P("keapi.chat/chat/completions", keapimiddleware.RequireAPIKeyPrivilege, keapiforestctl.ChatCompletions)

	// 对外检索接口
	eng.P("keapi.Search", keapimiddleware.RequireAPIKeyPrivilege, keapisearchctl.ForestSearch)
	eng.P("keapi.WhoAmI", keapimiddleware.RequireAPIKeyPrivilege, WhoAmI)
	eng.P("keapi.CLIAuthStart", CLIAuthStart)
	eng.P("keapi.CLIAuthInfo", CLIAuthInfo)
	eng.P("keapi.CLIAuthPoll", CLIAuthPoll)
	eng.P("keapi.RevokeCurrentAPIKey", keapimiddleware.RequireAPIKeyPrivilege, RevokeCurrentAPIKey)
}
