package dtokeapi

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/apis/errcode"
)

func TestValidCreateChatSessionAcceptsKnowledgeBaseScope(t *testing.T) {
	req := &CreateChatSessionRequest{}
	req.Request.ForestID = 20001
	resp := &apiobj.BaseResponse{}

	require.True(t, req.ValidCreateChatSession(resp))
	require.Equal(t, uint32(0), resp.Code)
}

func TestValidCreateChatSessionRejectsConflictingScopes(t *testing.T) {
	req := &CreateChatSessionRequest{}
	req.Request.ForestID = 20001
	req.Request.ForestFileIDs = []uint{30001}
	resp := &apiobj.BaseResponse{}

	require.False(t, req.ValidCreateChatSession(resp))
	require.Equal(t, uint32(errcode.ErrCode_BadRequest), resp.Code)
	require.Equal(t, "keapi_chat_scope_conflict", resp.Message)
}
