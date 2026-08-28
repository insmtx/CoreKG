package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/buildinfo"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testClientFactory(handler roundTripFunc) func(string, time.Duration) (*api.Client, error) {
	return func(serverURL string, timeout time.Duration) (*api.Client, error) {
		client, err := api.NewWithTimeout(serverURL, timeout)
		if err != nil {
			return nil, err
		}
		client.HTTPClient.Transport = handler
		return client, nil
	}
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    &http.Request{URL: &url.URL{Scheme: "https", Host: "corekg.example.com"}},
	}
}

func TestAskPersistsOnlyAfterSuccessfulAnswer(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	state := store.NewState()
	state.CurrentProfile = "work"
	state.Profiles["work"] = store.Profile{
		ServerURL:       "https://corekg.example.com",
		Credential:      "credential",
		KnowledgeBaseID: "42",
	}
	require.NoError(t, store.SaveState(paths, state))
	auth := store.NewAuth()
	auth.Credentials["credential"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "secret"}
	require.NoError(t, store.SaveAuth(paths, auth))

	var createCalls int
	factory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v3/keapi.BatchGetForest":
			return testResponse(http.StatusOK, `{"code":0,"response":{"total":1,"data":[{"forest_id":42,"name":"KB"}]}}`), nil
		case "/v3/keapi.CreateChat":
			createCalls++
			return testResponse(http.StatusOK, `{"code":0,"response":{"session_id":100,"forest_id":[42]}}`), nil
		case "/v3/keapi.chat/chat/completions":
			return testResponse(http.StatusOK, `{"id":"message-1","model":"model","choices":[{"message":{"role":"assistant","content":"answer"}}]}`), nil
		default:
			return testResponse(http.StatusNotFound, `{"error":{"message":"unexpected path"}}`), nil
		}
	})

	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:         paths,
		Out:           io.Discard,
		ErrOut:        io.Discard,
		ClientFactory: factory,
	})
	root.SetArgs([]string{"ask", "--output", "json", "first question"})
	require.NoError(t, root.Execute())

	loaded, err := store.LoadState(paths)
	require.NoError(t, err)
	require.Equal(t, uint(100), loaded.Profiles["work"].ChatSessions["42"])
	require.Equal(t, 1, createCalls)

	failingFactory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v3/keapi.BatchGetForest":
			return testResponse(http.StatusOK, `{"code":0,"response":{"total":1,"data":[{"forest_id":42,"name":"KB"}]}}`), nil
		case "/v3/keapi.CreateChat":
			return testResponse(http.StatusOK, `{"code":0,"response":{"session_id":101,"forest_id":[42]}}`), nil
		case "/v3/keapi.chat/chat/completions":
			return testResponse(http.StatusBadGateway, `{"error":{"code":"upstream","message":"model unavailable"}}`), nil
		default:
			return testResponse(http.StatusNotFound, `{"error":{"message":"unexpected path"}}`), nil
		}
	})
	root = NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:         paths,
		Out:           io.Discard,
		ErrOut:        io.Discard,
		ClientFactory: failingFactory,
	})
	root.SetArgs([]string{"ask", "--new", "--output", "json", "second question"})
	err = root.Execute()
	require.Error(t, err)
	loaded, loadErr := store.LoadState(paths)
	require.NoError(t, loadErr)
	require.Equal(t, uint(100), loaded.Profiles["work"].ChatSessions["42"])
	var cliError *clierr.Error
	require.True(t, errors.As(err, &cliError))
	details, ok := cliError.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, uint(101), details["session_id"])
}

func TestWaitForFileRetriesTemporaryNotFound(t *testing.T) {
	calls := 0
	clientFactory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			return testResponse(http.StatusOK, `{"code":0,"response":{"data":[]}}`), nil
		case 2:
			return testResponse(http.StatusOK, `{"code":0,"response":{"data":[{"forest_file_id":7,"file_status":"pending"}]}}`), nil
		default:
			return testResponse(http.StatusOK, `{"code":0,"response":{"data":[{"forest_file_id":7,"file_status":"success"}]}}`), nil
		}
	})
	client, err := clientFactory("https://corekg.example.com", time.Second)
	require.NoError(t, err)
	active := &activeProfile{Client: client, Credential: store.Credential{APIKey: "secret"}}

	file, err := (&app{}).waitForFileWithInterval(context.Background(), active, 7, time.Second, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, uint(7), file.ForestFileID)
	require.Equal(t, 3, calls)
}

func TestFileWaitErrorIncludesCreatedFileID(t *testing.T) {
	err := fileWaitCLIError(42, 7, &fileWaitError{
		code:   "file_wait_timeout",
		status: "pending",
		cause:  errors.New("file did not become ready"),
	})
	var cliError *clierr.Error
	require.True(t, errors.As(err, &cliError))
	require.Equal(t, "file_wait_timeout", cliError.Code)
	details, ok := cliError.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, uint(42), details["knowledge_base_id"])
	require.Equal(t, uint(7), details["forest_file_id"])
	require.Equal(t, "pending", details["file_status"])
}

func TestUploadFileStreamsMultipartFieldsAndContent(t *testing.T) {
	temporaryFile, err := os.CreateTemp(t.TempDir(), "upload-*.txt")
	require.NoError(t, err)
	_, err = temporaryFile.WriteString("hello CoreKG")
	require.NoError(t, err)
	require.NoError(t, temporaryFile.Close())
	temporaryFile, err = os.Open(temporaryFile.Name())
	require.NoError(t, err)
	defer temporaryFile.Close()

	var fields map[string]string
	var uploadedContent string
	factory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			return nil, errors.New("invalid multipart content type")
		}
		reader := multipart.NewReader(request.Body, params["boundary"])
		fields = make(map[string]string)
		for {
			part, nextErr := reader.NextPart()
			if errors.Is(nextErr, io.EOF) {
				break
			}
			if nextErr != nil {
				return nil, nextErr
			}
			data, readErr := io.ReadAll(part)
			if readErr != nil {
				return nil, readErr
			}
			if part.FormName() == "file" {
				uploadedContent = string(data)
			} else {
				fields[part.FormName()] = string(data)
			}
		}
		return testResponse(http.StatusOK, `{"code":0,"response":{"forest_file_id":7}}`), nil
	})
	client, err := factory("https://corekg.example.com", time.Second)
	require.NoError(t, err)
	active := &activeProfile{Client: client, Credential: store.Credential{APIKey: "secret"}}
	var result api.UploadFileResult
	require.NoError(t, uploadFile(context.Background(), active, 42, 9, "notes.txt", temporaryFile, &result))
	require.Equal(t, uint(7), result.ForestFileID)
	require.Equal(t, "42", fields["forest_id"])
	require.Equal(t, "9", fields["parent_id"])
	require.Equal(t, "hello CoreKG", uploadedContent)
}

func TestUploadCommandRejectsEmptyFileID(t *testing.T) {
	temporaryFile, err := os.CreateTemp(t.TempDir(), "upload-*.txt")
	require.NoError(t, err)
	_, err = temporaryFile.WriteString("hello")
	require.NoError(t, err)
	require.NoError(t, temporaryFile.Close())

	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	state := store.NewState()
	state.CurrentProfile = "work"
	state.Profiles["work"] = store.Profile{ServerURL: "https://corekg.example.com", Credential: "credential", KnowledgeBaseID: "42"}
	require.NoError(t, store.SaveState(paths, state))
	auth := store.NewAuth()
	auth.Credentials["credential"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "secret"}
	require.NoError(t, store.SaveAuth(paths, auth))

	factory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v3/keapi.BatchGetForest":
			return testResponse(http.StatusOK, `{"code":0,"response":{"total":1,"data":[{"forest_id":42,"name":"KB"}]}}`), nil
		case "/v3/keapi.UploadFile":
			_, _ = io.Copy(io.Discard, request.Body)
			return testResponse(http.StatusOK, `{"code":0,"response":{"forest_file_id":0}}`), nil
		default:
			return testResponse(http.StatusNotFound, `{"error":{"message":"unexpected path"}}`), nil
		}
	})
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{Paths: paths, Out: io.Discard, ErrOut: io.Discard, ClientFactory: factory})
	root.SetArgs([]string{"file", "upload", temporaryFile.Name()})
	err = root.Execute()
	var cliError *clierr.Error
	require.True(t, errors.As(err, &cliError))
	require.Equal(t, "file_upload_failed", cliError.Code)
}

func TestParseSessionIDUsesPlatformUintWidth(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("requires a 64-bit platform")
	}
	id, err := parseSessionID("4294967297")
	require.NoError(t, err)
	require.Equal(t, uint(4294967297), id)
}

func TestAuthReloginPreservesScopedProfileState(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	state := store.NewState()
	state.CurrentProfile = "work"
	state.Profiles["work"] = store.Profile{
		ServerURL:         "https://corekg.example.com",
		Credential:        "old-credential",
		OrganizationID:    "7",
		KnowledgeBaseID:   "42",
		KnowledgeBaseName: "KB",
		ChatSessions:      map[string]uint{"42": 100},
	}
	require.NoError(t, store.SaveState(paths, state))
	auth := store.NewAuth()
	auth.Credentials["old-credential"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "old"}
	require.NoError(t, store.SaveAuth(paths, auth))

	app := &app{}
	require.NoError(t, app.persistDeviceLogin(paths, "https://corekg.example.com", "work", api.CLIAuthPoll{
		APIKey:      "new",
		CompanyID:   7,
		CompanyName: "Acme",
	}, "device-code"))

	loaded, err := store.LoadState(paths)
	require.NoError(t, err)
	require.Equal(t, "42", loaded.Profiles["work"].KnowledgeBaseID)
	require.Equal(t, uint(100), loaded.Profiles["work"].ChatSessions["42"])
}

func TestBatchGetForestIsUsedForNumericKnowledgeBaseSelector(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	state := store.NewState()
	state.CurrentProfile = "work"
	state.Profiles["work"] = store.Profile{ServerURL: "https://corekg.example.com", Credential: "credential"}
	require.NoError(t, store.SaveState(paths, state))
	auth := store.NewAuth()
	auth.Credentials["credential"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "secret"}
	require.NoError(t, store.SaveAuth(paths, auth))

	called := ""
	factory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		called = request.URL.Path
		return testResponse(http.StatusOK, `{"code":0,"response":{"total":1,"data":[{"forest_id":42,"name":"KB"}]}}`), nil
	})
	active, err := (&app{paths: paths, clientFactory: factory}).loadActiveProfile("work")
	require.NoError(t, err)
	forest, err := (&app{paths: paths, clientFactory: factory}).resolveKnowledgeBase(context.Background(), active, "42")
	require.NoError(t, err)
	require.Equal(t, uint(42), forest.ForestID)
	require.Equal(t, "/v3/keapi.BatchGetForest", called)
}

func TestChatOperationErrorIncludesAnswerOnStateFailure(t *testing.T) {
	err := chatOperationError("chat_session_state_failed", errors.New("save state: disk full"), 42, 100, "answer")
	var cliError *clierr.Error
	require.True(t, errors.As(err, &cliError))
	detailsJSON, err := json.Marshal(cliError.Details)
	require.NoError(t, err)
	require.Contains(t, string(detailsJSON), "answer")
}

func TestKBCreatePromptsAndSelectsCreatedKnowledgeBase(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	state := store.NewState()
	state.CurrentProfile = "work"
	state.Profiles["work"] = store.Profile{
		ServerURL:         "https://corekg.example.com",
		Credential:        "credential",
		KnowledgeBaseID:   "42",
		KnowledgeBaseName: "Old KB",
		ChatSessions:      map[string]uint{"42": 100},
	}
	require.NoError(t, store.SaveState(paths, state))
	auth := store.NewAuth()
	auth.Credentials["credential"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "secret"}
	require.NoError(t, store.SaveAuth(paths, auth))

	var requestBody map[string]map[string]any
	factory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v3/keapi.CreateForest" {
			return testResponse(http.StatusNotFound, `{"error":{"message":"unexpected path"}}`), nil
		}
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			return nil, err
		}
		return testResponse(http.StatusOK, `{"code":0,"response":{"forest_id":99}}`), nil
	})
	var output, prompts bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:         paths,
		In:            strings.NewReader("New KB\nA short description\nhttps://example.com/avatar.png\ny\n"),
		Out:           &output,
		ErrOut:        &prompts,
		ClientFactory: factory,
	})
	root.SetArgs([]string{"kb", "create", "--use", "--output", "json"})
	require.NoError(t, root.Execute())

	require.Equal(t, "New KB", requestBody["request"]["name"])
	require.Equal(t, "A short description", requestBody["request"]["description"])
	require.Equal(t, "file", requestBody["request"]["forest_type"])
	require.Equal(t, "https://example.com/avatar.png", requestBody["request"]["avatar_url"])
	require.Contains(t, prompts.String(), "Create knowledge base?")
	require.Contains(t, output.String(), `"forest_id": 99`)
	require.NotContains(t, output.String(), `"selected"`)

	loaded, err := store.LoadState(paths)
	require.NoError(t, err)
	require.Equal(t, "99", loaded.Profiles["work"].KnowledgeBaseID)
	require.Equal(t, "New KB", loaded.Profiles["work"].KnowledgeBaseName)
	require.Equal(t, uint(100), loaded.Profiles["work"].ChatSessions["42"])
}

func TestKBCreateCancellationDoesNotCallAPI(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	state := store.NewState()
	state.CurrentProfile = "work"
	state.Profiles["work"] = store.Profile{ServerURL: "https://corekg.example.com", Credential: "credential", KnowledgeBaseID: "42"}
	require.NoError(t, store.SaveState(paths, state))
	auth := store.NewAuth()
	auth.Credentials["credential"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "secret"}
	require.NoError(t, store.SaveAuth(paths, auth))

	called := false
	factory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		called = true
		return testResponse(http.StatusOK, `{"code":0,"response":{"forest_id":99}}`), nil
	})
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:         paths,
		In:            strings.NewReader("New KB\n\n\nno\n"),
		Out:           io.Discard,
		ErrOut:        io.Discard,
		ClientFactory: factory,
	})
	root.SetArgs([]string{"kb", "create"})
	err := root.Execute()
	var cliError *clierr.Error
	require.True(t, errors.As(err, &cliError))
	require.Equal(t, "kb_create_cancelled", cliError.Code)
	require.Equal(t, clierr.ExitConfirm, clierr.ExitCode(err))
	require.False(t, called)

	loaded, loadErr := store.LoadState(paths)
	require.NoError(t, loadErr)
	require.Equal(t, "42", loaded.Profiles["work"].KnowledgeBaseID)
}

func TestKBCreateYesUsesDefaultsAndIDOutput(t *testing.T) {
	paths := store.NewPaths(filepath.Join(t.TempDir(), ".corekg"))
	state := store.NewState()
	state.CurrentProfile = "work"
	state.Profiles["work"] = store.Profile{ServerURL: "https://corekg.example.com", Credential: "credential"}
	require.NoError(t, store.SaveState(paths, state))
	auth := store.NewAuth()
	auth.Credentials["credential"] = store.Credential{ServerURL: "https://corekg.example.com", APIKey: "secret"}
	require.NoError(t, store.SaveAuth(paths, auth))

	var requestBody map[string]map[string]any
	factory := testClientFactory(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			return nil, err
		}
		return testResponse(http.StatusOK, `{"code":0,"response":{"forest_id":100}}`), nil
	})
	var output bytes.Buffer
	root := NewRootWithOptions(buildinfo.Info{Name: "corekg-cli", Version: "test"}, Options{
		Paths:         paths,
		In:            strings.NewReader("must not be read"),
		Out:           &output,
		ErrOut:        io.Discard,
		ClientFactory: factory,
	})
	root.SetArgs([]string{"kb", "create", "Automation", "--yes", "--output", "id"})
	require.NoError(t, root.Execute())
	require.Equal(t, "100\n", output.String())
	require.Equal(t, "Automation", requestBody["request"]["name"])
	require.Equal(t, "file", requestBody["request"]["forest_type"])
	require.Equal(t, "", requestBody["request"]["description"])
	require.Equal(t, "", requestBody["request"]["avatar_url"])
}
