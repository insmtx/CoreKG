package svcforestchat

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	aclopenai "github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/keapi/internal/dto/dtokeapi"
	chatcore "github.com/insmtx/corekg/apps/kechat/chat/core"
	chatwrapper "github.com/insmtx/corekg/apps/kechat/chat/wrapper"
	"github.com/insmtx/corekg/apps/kechat/models/chatmodel"
	"github.com/insmtx/corekg/apps/kechat/models/chatquestion"
	"github.com/insmtx/corekg/apps/kechat/models/chatsession"
	"github.com/insmtx/corekg/apps/kechat/models/chattype"
	"github.com/insmtx/corekg/apps/kecore/models/forest"
	"github.com/insmtx/corekg/apps/kecore/models/foresttype"
	"github.com/insmtx/corekg/apps/kecore/services/svcforest"
	"github.com/insmtx/corekg/apps/kellm/models/kellmtype"
	"github.com/insmtx/corekg/pkgs/einotools/printer"
	roctypes "github.com/insmtx/corekg/pkgs/types"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/apis/constants"
	"github.com/ygpkg/yg-go/apis/runtime"
	"github.com/ygpkg/yg-go/logs"
)

var (
	ErrInvalidChatMessages = errors.New("invalid chat messages")
	ErrInvalidForestFiles  = errors.New("invalid forest files")
	ErrInvalidForestScope  = errors.New("invalid forest scope")
	ErrChatModelNotFound   = errors.New("chat model not found")
)

const defaultChatCompletionsTemperature float32 = 0.2

type Result struct {
	ID     string
	Answer string
	Usage  kellmtype.Usage
}

type chatHistoryPair struct {
	Question string
	Answer   string
}

type messagePayload struct {
	Text     string
	Images   []string
	Combined string
}

type chatSessionPlan struct {
	Session           *chattype.ChatSession
	Model             *chattype.ChatModel
	Cleanup           func()
	SeedHistory       bool
	UpdateSessionName bool
}

type chatInputPlan struct {
	Question       string
	Images         []string
	SummaryPrompt  string
	QuestionEntity *chattype.ChatQuestion
}

func Run(ctx *gin.Context, req *dtokeapi.ChatCompletionsRequest) (*Result, error) {
	return RunWithPrinter(ctx, req, nil)
}

func RunWithPrinter(ctx *gin.Context, req *dtokeapi.ChatCompletionsRequest, msgPrinter printer.Printer) (*Result, error) {
	plan, err := prepareChatSessionPlan(ctx, req)
	if err != nil {
		return nil, err
	}
	if plan.Cleanup != nil {
		defer plan.Cleanup()
	}

	inputPlan, err := prepareChatMessages(ctx, plan, req.Request.Messages)
	if err != nil {
		return nil, err
	}

	questionEntity := inputPlan.QuestionEntity
	if questionEntity == nil {
		questionEntity, err = createCurrentQuestion(ctx, plan.Session, inputPlan.Question, inputPlan.Images)
		if err != nil {
			return nil, err
		}
	}
	resultID := dtokeapi.BuildChatSessionMessageID(questionEntity, "assistant")
	if openAIPrinter, ok := msgPrinter.(*OpenAIPrinter); ok {
		openAIPrinter.SetID(resultID)
	}

	extra := buildChatContextExtra(req, inputPlan, msgPrinter)

	_, err = chatwrapper.NewChatWrapper(ctx, &chatcore.ChatContext{
		Session:      plan.Session,
		Question:     questionEntity,
		Model:        plan.Model,
		ModelOptions: buildChatModelOptions(req),
		Extra:        extra,
	}).Run(ctx)
	if updateErr := finalizeChatQuestion(ctx, plan, questionEntity); updateErr != nil {
		return nil, updateErr
	}
	if err != nil && strings.TrimSpace(questionEntity.Source.Answer) == "" {
		return nil, err
	}

	if questionEntity.Source == nil {
		return nil, fmt.Errorf("chat question result is empty")
	}
	if questionEntity.Source.Status == chattype.QuestionStatusError && strings.TrimSpace(questionEntity.Source.Answer) == "" {
		return nil, fmt.Errorf("chat execution failed")
	}

	return &Result{
		ID:     resultID,
		Answer: normalizeAnswer(questionEntity.Source.Answer),
		Usage:  usageFromQuestion(questionEntity.Source, req.Request.Messages),
	}, nil
}

func buildChatContextExtra(req *dtokeapi.ChatCompletionsRequest, inputPlan *chatInputPlan, msgPrinter printer.Printer) map[string]any {
	extra := map[string]any{}
	if msgPrinter != nil {
		extra[chatcore.ExtraKeyPrinter] = msgPrinter
	}
	if inputPlan != nil && strings.TrimSpace(inputPlan.SummaryPrompt) != "" {
		extra[chatcore.ExtraKeySummarySystemPrompt] = strings.TrimSpace(inputPlan.SummaryPrompt)
	}
	if req != nil && req.Request.ExtraBody != nil && req.Request.ExtraBody.EnableReference != nil {
		extra[chatcore.ExtraKeyEnableReference] = *req.Request.ExtraBody.EnableReference
	}
	return extra
}

func buildChatModelOptions(req *dtokeapi.ChatCompletionsRequest) chatcore.ChatModelOptions {
	options := chatcore.ChatModelOptions{
		Temperature: float32Ptr(defaultChatCompletionsTemperature),
	}
	if req == nil {
		return options
	}
	if req.Request.Temperature != nil {
		options.Temperature = req.Request.Temperature
	}
	if req.Request.TopP != nil {
		options.TopP = req.Request.TopP
	}
	if req.Request.PresencePenalty != nil {
		options.PresencePenalty = req.Request.PresencePenalty
	}
	if req.Request.ResponseFormat != nil {
		options.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: aclopenai.ChatCompletionResponseFormatType(req.Request.ResponseFormat.Type),
		}
	}
	return options
}

func float32Ptr(v float32) *float32 {
	return &v
}

func prepareChatSessionPlan(ctx *gin.Context, req *dtokeapi.ChatCompletionsRequest) (*chatSessionPlan, error) {
	if req.Request.SessionID > 0 {
		return prepareExistingChatSessionPlan(ctx, req.Request.SessionID)
	}

	model, err := selectChatModel(ctx)
	if err != nil {
		return nil, err
	}
	session, cleanup, err := createOneShotSession(ctx, model, req.Request.ForestFileIDs)
	if err != nil {
		return nil, err
	}
	return &chatSessionPlan{
		Session:     session,
		Model:       model,
		Cleanup:     cleanup,
		SeedHistory: true,
	}, nil
}

func prepareExistingChatSessionPlan(ctx *gin.Context, sessionID uint) (*chatSessionPlan, error) {
	session, err := getOwnedChatSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	session.BaseType = chattype.ResourceQASessionBaseTypeForestAgent
	if strings.TrimSpace(session.EsIndex) == "" {
		session.EsIndex = "ke_0"
	}

	model, err := getSessionModel(ctx, session)
	if err != nil {
		return nil, err
	}
	return &chatSessionPlan{
		Session:           session,
		Model:             model,
		UpdateSessionName: true,
	}, nil
}

func prepareChatMessages(ctx *gin.Context, plan *chatSessionPlan, messages []kellmtype.Message) (*chatInputPlan, error) {
	if len(messages) == 0 {
		return prepareChatMessagesFromSession(ctx, plan)
	}

	if !plan.SeedHistory {
		currentQuestion, currentImages, summaryPrompt, err := buildCurrentChatInput(messages)
		if err != nil {
			return nil, err
		}
		return &chatInputPlan{
			Question:      currentQuestion,
			Images:        currentImages,
			SummaryPrompt: summaryPrompt,
		}, nil
	}

	historyPairs, currentQuestion, currentImages, summaryPrompt, err := buildChatInput(messages)
	if err != nil {
		return nil, err
	}
	if err := seedHistory(ctx, plan.Session, historyPairs); err != nil {
		return nil, err
	}
	return &chatInputPlan{
		Question:      currentQuestion,
		Images:        currentImages,
		SummaryPrompt: summaryPrompt,
	}, nil
}

func prepareChatMessagesFromSession(ctx *gin.Context, plan *chatSessionPlan) (*chatInputPlan, error) {
	if plan == nil || plan.Session == nil || plan.Session.ID == 0 {
		return nil, fmt.Errorf("%w: session_id is required when messages is empty", ErrInvalidChatMessages)
	}

	questions, err := chatquestion.ListSessionQuestionsByUin(ctx, plan.Session.Uin, plan.Session.ID)
	if err != nil {
		return nil, err
	}

	return buildChatInputFromSessionQuestions(questions)
}

func buildChatInputFromSessionQuestions(questions []*chattype.ChatQuestion) (*chatInputPlan, error) {
	var lastQuestion *chattype.ChatQuestion
	for i := len(questions) - 1; i >= 0; i-- {
		if questions[i] == nil || questions[i].Source == nil {
			continue
		}
		lastQuestion = questions[i]
		break
	}
	if lastQuestion == nil || strings.TrimSpace(lastQuestion.Source.Question) == "" {
		return nil, fmt.Errorf("%w: last user message is required", ErrInvalidChatMessages)
	}
	if lastQuestion.Source.Status == chattype.QuestionStatusAnswered || strings.TrimSpace(lastQuestion.Source.Answer) != "" {
		return nil, fmt.Errorf("%w: last session message must be an unanswered user message", ErrInvalidChatMessages)
	}

	return &chatInputPlan{
		Question:       lastQuestion.Source.Question,
		Images:         lastQuestion.Source.ImageUrlList,
		QuestionEntity: lastQuestion,
	}, nil
}

func finalizeChatQuestion(ctx *gin.Context, plan *chatSessionPlan, questionEntity *chattype.ChatQuestion) error {
	if questionEntity == nil || questionEntity.Source == nil {
		return fmt.Errorf("chat question result is empty")
	}

	logs.InfoContextf(ctx, "[keapi.ChatCompletions] questionEntity: %s", logs.JSON(questionEntity))
	if err := chatquestion.UpdateQuestion(ctx, questionEntity); err != nil {
		logs.ErrorContextf(ctx, "[keapi.ChatCompletions] Failed to update question, questionEntity: %s, err: %v", logs.JSON(questionEntity), err)
		return err
	}
	if plan.UpdateSessionName && strings.TrimSpace(questionEntity.Source.Answer) != "" {
		updateSessionNameWithLLMAsync(ctx, plan.Session, questionEntity.Source.Question, questionEntity.Source.Answer)
	}
	return nil
}

func updateSessionNameWithLLMAsync(ctx *gin.Context, session *chattype.ChatSession, question, answer string) {
	if session == nil || strings.TrimSpace(question) == "" || strings.TrimSpace(answer) == "" {
		return
	}

	sessionID := session.ID
	uin := session.Uin
	requestID := ctx.GetString(constants.CtxKeyRequestID)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		bgCtx = logs.WithContextFields(bgCtx,
			"request_id", requestID,
			"session_id", sessionID,
			"uin", uin,
		)
		defer func() {
			if r := recover(); r != nil {
				logs.ErrorContextf(bgCtx, "[keapi.ChatCompletions] async UpdateSessionNameWithLLM panic: %v", r)
			}
		}()

		latestSession, err := chatsession.GetChatSession(bgCtx, uin, sessionID)
		if err != nil {
			logs.ErrorContextf(bgCtx, "[keapi.ChatCompletions] get session for async name update failed: %v", err)
			return
		}
		chatsession.UpdateSessionNameWithLLM(bgCtx, latestSession, question, answer)
	}()
}

func selectChatModel(ctx *gin.Context) (*chattype.ChatModel, error) {
	modelList := &chatmodel.QueryModelListResponse{}
	err := chatmodel.QueryModelList(ctx, apiobj.PageQuery{
		Uin:       runtime.Uin(ctx),
		CompanyID: runtime.CompanyID(ctx),
		Limit:     1,
	}, modelList)
	if err != nil {
		return nil, err
	}
	if len(modelList.Data) == 0 {
		return nil, ErrChatModelNotFound
	}
	return chatmodel.GetModelByID(ctx, modelList.Data[0].ID)
}

func createOneShotSession(ctx *gin.Context, model *chattype.ChatModel, fileIDs []uint) (*chattype.ChatSession, func(), error) {
	session, err := buildForestFileSession(ctx, model, fileIDs, "")
	if err != nil {
		return nil, nil, err
	}
	if err := chatsession.CreateSession(ctx, session); err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		if err := chatquestion.DeleteSessionQuestions(ctx, session.Uin, session.ID); err != nil {
			logs.ErrorContextf(ctx, "delete temporary chat questions failed: %v", err)
		}
		if err := chatsession.DeleteSession(ctx, session.ID); err != nil {
			logs.ErrorContextf(ctx, "delete temporary chat session failed: %v", err)
		}
	}
	return session, cleanup, nil
}

func buildForestFileSession(ctx *gin.Context, model *chattype.ChatModel, fileIDs []uint, name string) (*chattype.ChatSession, error) {
	forestIDs, esIndex, err := resolveForestFileScope(ctx, fileIDs)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = chattype.DefaultSessionName
	}

	session := &chattype.ChatSession{
		CompanyID:    runtime.CompanyID(ctx),
		Uin:          runtime.Uin(ctx),
		Name:         name,
		ModelName:    model.ShowName,
		ModelID:      model.ID,
		ResourceType: chattype.ResourceTypeFileList,
		BaseType:     chattype.ResourceQASessionBaseTypeForestAgent,
		FileIDList:   roctypes.NewUintArray(fileIDs),
		ForestIDList: roctypes.NewUintArray(forestIDs),
		EsIndex:      esIndex,
		PromptMode:   "normal",
	}
	return session, nil
}

func buildForestSession(ctx *gin.Context, model *chattype.ChatModel, forestID uint, name string) (*chattype.ChatSession, error) {
	forestList, err := svcforest.ListForest(ctx, &svcforest.ListForestRequest{
		Uin:       runtime.Uin(ctx),
		CompanyID: runtime.CompanyID(ctx),
		Query: apiobj.PageQuery{
			ListAll: true,
			Filters: []apiobj.Filter{{
				Field: "id",
				Value: []string{strconv.FormatUint(uint64(forestID), 10)},
			}},
		},
		PresetWhenListing: false,
	})
	if err != nil {
		return nil, fmt.Errorf("validate forest access: %w", err)
	}
	if len(forestList.Data) != 1 || forestList.Data[0].ID != forestID {
		return nil, fmt.Errorf("%w: forest %d is not accessible", ErrInvalidForestScope, forestID)
	}

	forestInfo := forestList.Data[0]
	name = strings.TrimSpace(name)
	if name == "" {
		name = chattype.DefaultSessionName
	}

	return &chattype.ChatSession{
		CompanyID:    runtime.CompanyID(ctx),
		Uin:          runtime.Uin(ctx),
		Name:         name,
		ModelName:    model.ShowName,
		ModelID:      model.ID,
		ResourceType: chattype.ResourceTypeForest,
		BaseType:     chattype.ResourceQASessionBaseTypeForestAgent,
		ForestIDList: roctypes.NewUintArray([]uint{forestID}),
		EsIndex:      forestInfo.EsIndex(),
		PromptMode:   "normal",
	}, nil
}

func resolveForestFileScope(ctx *gin.Context, fileIDs []uint) ([]uint, string, error) {
	if len(fileIDs) == 0 {
		return nil, "ke_0", nil
	}

	files, err := forest.GetForestFileByIDs(fileIDs)
	if err != nil {
		return nil, "", err
	}
	if len(files) == 0 {
		return nil, "", fmt.Errorf("%w: forest files not found", ErrInvalidForestFiles)
	}

	requestedFileIDs := make(map[uint]struct{}, len(fileIDs))
	for _, fileID := range fileIDs {
		requestedFileIDs[fileID] = struct{}{}
	}
	foundFileIDs := make(map[uint]struct{}, len(files))
	forestIDs := make([]uint, 0, len(files))
	forestIDSet := make(map[uint]struct{})
	var esIndex string

	for _, file := range files {
		foundFileIDs[file.ID] = struct{}{}
		if file.IsDir == 1 {
			return nil, "", fmt.Errorf("%w: forest_file_id contains directory", ErrInvalidForestFiles)
		}
		if file.KnowledgeStatus != foresttype.TaskStatusSuccess {
			return nil, "", fmt.Errorf("%w: forest file %d is not ready", ErrInvalidForestFiles, file.ID)
		}

		forestInfo, err := forest.GetForestByID(ctx, file.ForestID)
		if err != nil {
			return nil, "", err
		}
		currentIndex := forestInfo.EsIndex()
		if esIndex == "" {
			esIndex = currentIndex
		} else if esIndex != currentIndex {
			return nil, "", fmt.Errorf("%w: forest files span multiple es indexes", ErrInvalidForestFiles)
		}

		if _, ok := forestIDSet[file.ForestID]; !ok {
			forestIDSet[file.ForestID] = struct{}{}
			forestIDs = append(forestIDs, file.ForestID)
		}
	}

	if len(foundFileIDs) != len(requestedFileIDs) {
		return nil, "", fmt.Errorf("%w: some forest_file_id values do not exist", ErrInvalidForestFiles)
	}
	accessibleForests, err := svcforest.ListForest(ctx, &svcforest.ListForestRequest{
		Uin:       runtime.Uin(ctx),
		CompanyID: runtime.CompanyID(ctx),
		Query: apiobj.PageQuery{
			ListAll: true,
			Filters: []apiobj.Filter{{
				Field: "id",
				Value: forestIDValues(forestIDs),
			}},
		},
		PresetWhenListing: false,
	})
	if err != nil {
		return nil, "", fmt.Errorf("validate forest access: %w", err)
	}
	if len(accessibleForests.Data) != len(forestIDSet) {
		return nil, "", fmt.Errorf("%w: forest file is not accessible", ErrInvalidForestFiles)
	}
	if esIndex == "" {
		esIndex = "ke_0"
	}
	return forestIDs, esIndex, nil
}

func forestIDValues(forestIDs []uint) []string {
	values := make([]string, 0, len(forestIDs))
	for _, forestID := range forestIDs {
		values = append(values, strconv.FormatUint(uint64(forestID), 10))
	}
	return values
}

func buildChatInput(messages []kellmtype.Message) ([]chatHistoryPair, string, []string, string, error) {
	systemPrompts := make([]string, 0)
	historyPairs := make([]chatHistoryPair, 0)
	var pendingUser *messagePayload

	for _, message := range messages {
		payload := extractMessagePayload(message)
		if payload.Combined == "" {
			continue
		}

		switch message.Role {
		case "system":
			systemPrompts = append(systemPrompts, payload.Combined)
		case "user":
			if pendingUser != nil {
				return nil, "", nil, "", fmt.Errorf("%w: consecutive user messages are not supported", ErrInvalidChatMessages)
			}
			pendingUser = payload
		case "assistant":
			if pendingUser == nil {
				return nil, "", nil, "", fmt.Errorf("%w: assistant message must follow a user message", ErrInvalidChatMessages)
			}
			historyPairs = append(historyPairs, chatHistoryPair{
				Question: pendingUser.Combined,
				Answer:   payload.Combined,
			})
			pendingUser = nil
		}
	}

	if pendingUser == nil || pendingUser.Combined == "" {
		return nil, "", nil, "", fmt.Errorf("%w: last user message is required", ErrInvalidChatMessages)
	}

	return historyPairs, pendingUser.Combined, pendingUser.Images, strings.Join(systemPrompts, "\n"), nil
}

func buildCurrentChatInput(messages []kellmtype.Message) (string, []string, string, error) {
	systemPrompts := make([]string, 0)
	var currentUser *messagePayload

	for _, message := range messages {
		payload := extractMessagePayload(message)
		if payload.Combined == "" {
			continue
		}

		switch message.Role {
		case "system":
			systemPrompts = append(systemPrompts, payload.Combined)
		case "user":
			currentUser = payload
		}
	}

	if currentUser == nil || currentUser.Combined == "" {
		return "", nil, "", fmt.Errorf("%w: user message is required", ErrInvalidChatMessages)
	}

	return currentUser.Combined, currentUser.Images, strings.Join(systemPrompts, "\n"), nil
}

func extractMessagePayload(message kellmtype.Message) *messagePayload {
	textParts := make([]string, 0)
	imageURLs := make([]string, 0)

	if strings.TrimSpace(message.Content.Text) != "" {
		textParts = append(textParts, strings.TrimSpace(message.Content.Text))
	}

	for _, item := range message.Content.Items {
		if strings.TrimSpace(item.Text) != "" {
			textParts = append(textParts, strings.TrimSpace(item.Text))
		}
		if item.ImageURL != nil && strings.TrimSpace(item.ImageURL.URL) != "" {
			imageURL := strings.TrimSpace(item.ImageURL.URL)
			imageURLs = append(imageURLs, imageURL)
		}
	}

	combinedParts := make([]string, 0, len(textParts)+len(imageURLs))
	combinedParts = append(combinedParts, textParts...)
	combinedParts = append(combinedParts, imageURLs...)

	return &messagePayload{
		Text:     strings.Join(textParts, "\n"),
		Images:   imageURLs,
		Combined: strings.Join(combinedParts, "\n"),
	}
}

func seedHistory(ctx *gin.Context, session *chattype.ChatSession, historyPairs []chatHistoryPair) error {
	for index, pair := range historyPairs {
		questionEntity := &chattype.ChatQuestion{
			Source: &chattype.Question{
				ReqID:     fmt.Sprintf("%s-history-%d", runtime.RequestID(ctx), index),
				CompanyID: session.CompanyID,
				Uin:       session.Uin,
				ModelID:   session.ModelID,
				SessionID: session.ID,
				Question:  pair.Question,
				Answer:    pair.Answer,
				Status:    chattype.QuestionStatusAnswered,
			},
		}
		if err := chatquestion.CreateQuestion(ctx, questionEntity); err != nil {
			return err
		}
	}
	return nil
}

func createCurrentQuestion(ctx *gin.Context, session *chattype.ChatSession, question string, imageURLs []string) (*chattype.ChatQuestion, error) {
	questionEntity := &chattype.ChatQuestion{
		Source: &chattype.Question{
			ReqID:        runtime.RequestID(ctx),
			CompanyID:    session.CompanyID,
			Uin:          session.Uin,
			ModelID:      session.ModelID,
			SessionID:    session.ID,
			Question:     question,
			ImageUrlList: imageURLs,
			Status:       chattype.QuestionStatusPending,
		},
	}
	if err := chatquestion.CreateQuestion(ctx, questionEntity); err != nil {
		return nil, err
	}
	return questionEntity, nil
}

func usageFromQuestion(question *chattype.Question, messages []kellmtype.Message) kellmtype.Usage {
	if question != nil && question.TotalTokens > 0 {
		promptTokens := question.TotalTokens - question.OutToken
		if promptTokens < 0 {
			promptTokens = 0
		}
		return kellmtype.Usage{
			PromptTokens:          promptTokens,
			CompletionTokens:      question.OutToken,
			TotalTokens:           question.TotalTokens,
			PromptCacheHitTokens:  question.CacheHitToken,
			PromptCacheMissTokens: question.CacheMissToken,
		}
	}
	answer := ""
	if question != nil {
		answer = question.Answer
	}
	return approxUsage(messages, answer)
}

func approxUsage(messages []kellmtype.Message, answer string) kellmtype.Usage {
	promptChars := 0
	for _, message := range messages {
		promptChars += len(extractMessagePayload(message).Combined)
	}
	completionChars := len(answer)

	promptTokens := promptChars / 4
	if promptTokens == 0 && promptChars > 0 {
		promptTokens = 1
	}
	completionTokens := completionChars / 4
	if completionTokens == 0 && completionChars > 0 {
		completionTokens = 1
	}

	return kellmtype.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}
