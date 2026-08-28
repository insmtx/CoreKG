package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/output"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/spf13/cobra"
)

const maxQuestionBytes = 1 << 20

type askOutput struct {
	KnowledgeBaseID uint           `json:"knowledge_base_id"`
	SessionID       uint           `json:"session_id"`
	MessageID       string         `json:"message_id"`
	Answer          string         `json:"answer"`
	Model           string         `json:"model,omitempty"`
	Usage           map[string]any `json:"usage,omitempty"`
}

func (a *app) askCommand() *cobra.Command {
	var selector string
	var sessionIDValue string
	var promptFile string
	var newSession bool
	var askTimeout time.Duration
	command := &cobra.Command{
		Use:   "ask [QUESTION...]",
		Short: "Ask a question about a knowledge base",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "-" {
				return nil
			}
			if len(args) == 0 && strings.TrimSpace(promptFile) == "" {
				return clierr.Usage("question_required", "provide a question, use `-` for stdin, or pass --prompt-file")
			}
			if len(args) > 0 && strings.TrimSpace(promptFile) != "" {
				return clierr.Usage("question_input_conflict", "a question argument cannot be combined with --prompt-file")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("ask-timeout") && askTimeout <= 0 {
				return clierr.Usage("invalid_ask_timeout", "--ask-timeout must be positive")
			}
			if newSession && strings.TrimSpace(sessionIDValue) != "" {
				return clierr.Usage("session_flag_conflict", "--new cannot be combined with --session-id")
			}
			question, err := a.readQuestion(args, promptFile)
			if err != nil {
				return err
			}
			active, err := a.loadActiveProfileForOperation(a.profileName, 2*time.Minute, askTimeout)
			if err != nil {
				return err
			}
			forest, err := a.resolveKnowledgeBase(cmd.Context(), active, selector)
			if err != nil {
				return err
			}

			requestedSessionID, err := parseSessionID(sessionIDValue)
			if err != nil {
				return err
			}
			session, found, err := a.findChatSession(cmd.Context(), active, requestedSessionID, forest.ForestID)
			if err != nil {
				return err
			}
			if requestedSessionID > 0 && !found {
				return clierr.New("session_not_found", fmt.Sprintf("session %d does not belong to knowledge base %d or is no longer available", requestedSessionID, forest.ForestID))
			}

			if !found && !newSession {
				cachedSessionID := active.Definition.ChatSessions[strconv.FormatUint(uint64(forest.ForestID), 10)]
				if cachedSessionID > 0 {
					session, found, err = a.findChatSession(cmd.Context(), active, cachedSessionID, forest.ForestID)
					if err != nil {
						return err
					}
				}
			}
			if !found {
				session, err = a.createChatSession(cmd.Context(), active, forest.ForestID)
				if err != nil {
					return clierr.Wrap("chat_session_create_failed", err)
				}
			}

			completion, err := active.Client.ChatCompletion(cmd.Context(), active.Credential.APIKey, map[string]any{
				"session_id": session.SessionID,
				"messages": []map[string]string{{
					"role":    "user",
					"content": question,
				}},
				"stream": false,
				"extra_body": map[string]any{
					"enable_reference": true,
				},
			})
			if err != nil {
				return chatOperationError("ask_failed", err, forest.ForestID, session.SessionID, "")
			}
			answer := strings.TrimSpace(completion.Choices[0].Message.Content)
			if answer == "" {
				return chatOperationError("empty_answer", errors.New("the knowledge base returned an empty answer"), forest.ForestID, session.SessionID, "")
			}
			if err := a.saveChatSession(active, forest.ForestID, session.SessionID); err != nil {
				return chatOperationError("chat_session_state_failed", err, forest.ForestID, session.SessionID, answer)
			}
			return a.writeAskOutput(askOutput{
				KnowledgeBaseID: forest.ForestID,
				SessionID:       session.SessionID,
				MessageID:       completion.ID,
				Answer:          answer,
				Model:           completion.Model,
				Usage:           completion.Usage,
			})
		},
	}
	command.Flags().StringVar(&selector, "kb", "", "Knowledge base ID or exact name (default: selected knowledge base)")
	command.Flags().StringVar(&sessionIDValue, "session-id", "", "Resume this chat session explicitly")
	command.Flags().BoolVar(&newSession, "new", false, "Start a new chat session for this knowledge base")
	command.Flags().StringVar(&promptFile, "prompt-file", "", "Read the question from a file, or - for stdin")
	command.Flags().DurationVar(&askTimeout, "ask-timeout", 0, "HTTP timeout for this ask operation (default: 2m or longer configured timeout)")
	return command
}

func (a *app) readQuestion(args []string, promptFile string) (string, error) {
	var reader io.Reader
	switch {
	case strings.TrimSpace(promptFile) == "" && len(args) == 1 && args[0] == "-":
		reader = a.in
	case strings.TrimSpace(promptFile) != "":
		if promptFile == "-" {
			reader = a.in
			break
		}
		file, err := os.Open(promptFile)
		if err != nil {
			return "", clierr.Usage("invalid_prompt_file", fmt.Sprintf("open prompt file %q: %v", promptFile, err))
		}
		defer file.Close()
		reader = file
	default:
		return nonEmptyQuestion(strings.Join(args, " "))
	}

	question, err := readLimitedQuestion(reader)
	if err != nil {
		return "", clierr.Usage("invalid_question", err.Error())
	}
	return nonEmptyQuestion(question)
}

func readLimitedQuestion(reader io.Reader) (string, error) {
	if reader == nil {
		return "", fmt.Errorf("question input is not available")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxQuestionBytes+1))
	if err != nil {
		return "", fmt.Errorf("read question: %w", err)
	}
	if len(data) > maxQuestionBytes {
		return "", fmt.Errorf("question exceeds %d bytes", maxQuestionBytes)
	}
	return string(data), nil
}

func nonEmptyQuestion(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", clierr.Usage("question_required", "question must not be empty")
	}
	return value, nil
}

func parseSessionID(value string) (uint, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, strconv.IntSize)
	if err != nil || parsed == 0 {
		return 0, clierr.Usage("invalid_session_id", fmt.Sprintf("session ID %q must be a positive integer", value))
	}
	return uint(parsed), nil
}

func chatOperationError(code string, err error, forestID, sessionID uint, answer string) error {
	details := map[string]any{
		"knowledge_base_id": forestID,
		"session_id":        sessionID,
	}
	if answer != "" {
		details["answer"] = answer
	}
	var detailer interface{ CLIErrorDetails() any }
	if errors.As(err, &detailer) {
		if values, ok := detailer.CLIErrorDetails().(map[string]any); ok {
			for key, value := range values {
				details[key] = value
			}
		}
	}
	return clierr.WithDetails(code, fmt.Sprintf("knowledge base %d session %d: %s", forestID, sessionID, err), details)
}

func (a *app) findChatSession(ctx context.Context, active *activeProfile, sessionID, forestID uint) (api.ChatSession, bool, error) {
	if sessionID == 0 {
		return api.ChatSession{}, false, nil
	}
	var page api.ChatSessionPage
	if err := active.Client.DoJSON(ctx, active.Credential.APIKey, "keapi.BatchGetChatInfo", map[string]any{
		"session_ids": []uint{sessionID},
	}, &page); err != nil {
		return api.ChatSession{}, false, clierr.Wrap("chat_session_lookup_failed", err)
	}
	for _, session := range page.Data {
		if session.SessionID == sessionID && len(session.ForestIDs) == 1 && session.ForestIDs[0] == forestID {
			return session, true, nil
		}
	}
	return api.ChatSession{}, false, nil
}

func (a *app) createChatSession(ctx context.Context, active *activeProfile, forestID uint) (api.ChatSession, error) {
	var session api.ChatSession
	if err := active.Client.DoJSON(ctx, active.Credential.APIKey, "keapi.CreateChat", map[string]any{
		"forest_id": forestID,
		"name":      "corekg-cli",
	}, &session); err != nil {
		return api.ChatSession{}, err
	}
	if session.SessionID == 0 {
		return api.ChatSession{}, fmt.Errorf("keapi.CreateChat returned an empty session ID")
	}
	return session, nil
}

func (a *app) saveChatSession(active *activeProfile, forestID, sessionID uint) error {
	return store.WithLock(active.Paths, func() error {
		state, err := store.LoadState(active.Paths)
		if err != nil {
			return err
		}
		profile, ok := state.Profiles[active.Name]
		if !ok {
			return fmt.Errorf("profile %q does not exist", active.Name)
		}
		if profile.ChatSessions == nil {
			profile.ChatSessions = map[string]uint{}
		}
		profile.ChatSessions[strconv.FormatUint(uint64(forestID), 10)] = sessionID
		state.Profiles[active.Name] = profile
		return store.SaveState(active.Paths, state)
	})
}

func (a *app) writeAskOutput(value askOutput) error {
	format, err := a.format()
	if err != nil {
		return err
	}
	if format == "json" {
		return output.WriteJSON(a.out, value)
	}
	if format == "id" {
		return output.WriteNames(a.out, []string{strconv.FormatUint(uint64(value.SessionID), 10)})
	}
	if _, err := fmt.Fprintf(a.out, "Session: %d\n\n%s\n", value.SessionID, value.Answer); err != nil {
		return err
	}
	return nil
}
