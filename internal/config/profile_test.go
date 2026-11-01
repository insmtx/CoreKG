package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadPersonal(t *testing.T) {
	p, err := Load("../../configs/profiles/personal.yaml")
	require.NoError(t, err)
	require.NoError(t, p.Validate())
	require.Equal(t, "postgres", p.Infra.DB.Driver)
	require.Equal(t, "pgvector", p.Infra.Vector.Driver)
}

func TestValidateRejectsMySQL(t *testing.T) {
	p := validProfile()
	p.Infra.DB.Driver = "mysql"
	require.ErrorContains(t, p.Validate(), "infra.db.driver")
}

func TestValidateRejectsUnknownVector(t *testing.T) {
	p := validProfile()
	p.Infra.Vector.Driver = "sqlite_vec"
	require.ErrorContains(t, p.Validate(), "infra.vector.driver")
}

func TestValidateRequiresDSN(t *testing.T) {
	p := validProfile()
	p.Infra.DB.DSN = ""
	require.ErrorContains(t, p.Validate(), "infra.db.dsn")
}

func TestValidateRequiresLLMForChat(t *testing.T) {
	p := validProfile()
	p.Capabilities.Chat = true
	p.Infra.LLM.Driver = ""
	require.ErrorContains(t, p.Validate(), "infra.llm.driver")
}

func TestValidateRejectsUnknownProfile(t *testing.T) {
	p := validProfile()
	p.Profile = "team"
	require.ErrorContains(t, p.Validate(), "profile")
}

func TestLoadAIO(t *testing.T) {
	p, err := Load("../../configs/profiles/aio.yaml")
	require.NoError(t, err)
	require.NoError(t, p.Validate())
	require.Equal(t, "aio", p.Profile)
	require.Equal(t, "redis", p.Infra.Cache.Driver)
	require.Equal(t, "redis", p.Infra.Lock.Driver)
	require.Equal(t, "redis", p.Infra.Stream.Driver)
	require.Equal(t, "127.0.0.1:6379", p.Infra.Cache.Addr)
	require.True(t, p.Capabilities.MCP)
}

func TestLoadEnterprise(t *testing.T) {
	p, err := Load("../../configs/profiles/enterprise.yaml")
	require.NoError(t, err)
	require.NoError(t, p.Validate())
	require.Equal(t, "enterprise", p.Profile)
	require.Equal(t, "elasticsearch", p.Infra.Vector.Driver)
	require.Equal(t, "elasticsearch", p.Infra.Keyword.Driver)
	require.Equal(t, "s3", p.Infra.Object.Driver)
	require.Equal(t, "nats", p.Infra.Queue.Driver)
	require.Equal(t, "nebulagraph", p.Infra.Graph.Driver)
	require.True(t, p.Capabilities.Chat)
	require.True(t, p.Capabilities.Graph)
}

func TestValidate(t *testing.T) {
	t.Run("valid profiles load and validate", func(t *testing.T) {
		for _, name := range []string{"personal", "aio", "enterprise"} {
			p, err := Load(filepath.Join("..", "..", "configs", "profiles", name+".yaml"))
			require.NoError(t, err, name)
			require.NoError(t, p.Validate(), name)
		}
	})

	tests := []struct {
		name    string
		mutate  func(*Profile)
		wantErr string
	}{
		{name: "unknown profile name", mutate: func(p *Profile) { p.Profile = "team" }, wantErr: "profile"},
		{name: "db driver not postgres", mutate: func(p *Profile) { p.Infra.DB.Driver = "mysql" }, wantErr: "infra.db.driver"},
		{name: "db dsn empty", mutate: func(p *Profile) { p.Infra.DB.DSN = "" }, wantErr: "infra.db.dsn"},
		{name: "vector unknown driver", mutate: func(p *Profile) { p.Infra.Vector.Driver = "sqlite_vec" }, wantErr: "infra.vector.driver"},
		{name: "vector elasticsearch missing addr", mutate: func(p *Profile) {
			p.Infra.Vector.Driver = "elasticsearch"
			p.Infra.Vector.Addr = ""
			p.Infra.Vector.URL = ""
		}, wantErr: "infra.vector.addr"},
		{name: "keyword unknown driver", mutate: func(p *Profile) { p.Infra.Keyword.Driver = "meilisearch" }, wantErr: "infra.keyword.driver"},
		{name: "keyword elasticsearch missing addr", mutate: func(p *Profile) {
			p.Infra.Keyword.Driver = "elasticsearch"
			p.Infra.Keyword.Addr = ""
		}, wantErr: "infra.keyword.addr"},
		{name: "object unknown driver", mutate: func(p *Profile) { p.Infra.Object.Driver = "oss" }, wantErr: "infra.object.driver"},
		{name: "object localdir missing root", mutate: func(p *Profile) {
			p.Infra.Object.Driver = "localdir"
			p.Infra.Object.Root = ""
		}, wantErr: "infra.object.root"},
		{name: "object s3 missing url", mutate: func(p *Profile) {
			p.Infra.Object.Driver = "s3"
			p.Infra.Object.URL = ""
		}, wantErr: "infra.object.url"},
		{name: "object s3 missing bucket", mutate: func(p *Profile) {
			p.Infra.Object.Driver = "s3"
			p.Infra.Object.URL = "http://127.0.0.1:9000"
			p.Infra.Object.Bucket = ""
		}, wantErr: "infra.object.bucket"},
		{name: "cache unknown driver", mutate: func(p *Profile) { p.Infra.Cache.Driver = "memcached" }, wantErr: "infra.cache.driver"},
		{name: "cache redis missing addr", mutate: func(p *Profile) {
			p.Infra.Cache.Driver = "redis"
			p.Infra.Cache.Addr = ""
		}, wantErr: "infra.cache.addr"},
		{name: "lock unknown driver", mutate: func(p *Profile) { p.Infra.Lock.Driver = "etcd" }, wantErr: "infra.lock.driver"},
		{name: "lock redis missing addr", mutate: func(p *Profile) { p.Infra.Lock.Driver = "redis"; p.Infra.Lock.Addr = "" }, wantErr: "infra.lock.addr"},
		{name: "stream unknown driver", mutate: func(p *Profile) { p.Infra.Stream.Driver = "kafka" }, wantErr: "infra.stream.driver"},
		{name: "stream redis missing addr", mutate: func(p *Profile) { p.Infra.Stream.Driver = "redis"; p.Infra.Stream.Addr = "" }, wantErr: "infra.stream.addr"},
		{name: "queue unknown driver", mutate: func(p *Profile) { p.Infra.Queue.Driver = "kafka" }, wantErr: "infra.queue.driver"},
		{name: "queue nats missing url", mutate: func(p *Profile) { p.Infra.Queue.Driver = "nats"; p.Infra.Queue.URL = "" }, wantErr: "infra.queue.url"},
		{name: "embedding unknown driver", mutate: func(p *Profile) { p.Infra.Embedding.Driver = "ollama" }, wantErr: "infra.embedding.driver"},
		{name: "embedding http missing url", mutate: func(p *Profile) {
			p.Infra.Embedding.Driver = "http"
			p.Infra.Embedding.URL = ""
		}, wantErr: "infra.embedding.url"},
		{name: "rerank unknown driver", mutate: func(p *Profile) { p.Infra.Rerank.Driver = "bge" }, wantErr: "infra.rerank.driver"},
		{name: "rerank http missing url", mutate: func(p *Profile) { p.Infra.Rerank.Driver = "http"; p.Infra.Rerank.URL = "" }, wantErr: "infra.rerank.url"},
		{name: "llm unknown driver", mutate: func(p *Profile) { p.Infra.LLM.Driver = "anthropic" }, wantErr: "infra.llm.driver"},
		{name: "llm http missing url", mutate: func(p *Profile) { p.Infra.LLM.Driver = "http"; p.Infra.LLM.URL = "" }, wantErr: "infra.llm.url"},
		{name: "llm openai missing url", mutate: func(p *Profile) { p.Infra.LLM.Driver = "openai"; p.Infra.LLM.URL = "" }, wantErr: "infra.llm.url"},
		{name: "parser unknown driver", mutate: func(p *Profile) { p.Infra.Parser.Driver = "tika" }, wantErr: "infra.parser.driver"},
		{name: "parser http missing url", mutate: func(p *Profile) { p.Infra.Parser.Driver = "http"; p.Infra.Parser.URL = "" }, wantErr: "infra.parser.url"},
		{name: "graph unknown driver", mutate: func(p *Profile) { p.Infra.Graph.Driver = "neo4j" }, wantErr: "infra.graph.driver"},
		{name: "graph nebulagraph missing addr", mutate: func(p *Profile) {
			p.Infra.Graph.Driver = "nebulagraph"
			p.Infra.Graph.Addr = ""
		}, wantErr: "infra.graph.addr"},
		{name: "chat requires llm", mutate: func(p *Profile) { p.Capabilities.Chat = true; p.Infra.LLM.Driver = "" }, wantErr: "infra.llm.driver"},
		{name: "search requires vector", mutate: func(p *Profile) { p.Capabilities.Search = true; p.Infra.Vector.Driver = "" }, wantErr: "infra.vector.driver"},
		{name: "search requires embedding", mutate: func(p *Profile) { p.Capabilities.Search = true; p.Infra.Embedding.Driver = "" }, wantErr: "infra.embedding.driver"},
		{name: "graph capability with none driver", mutate: func(p *Profile) { p.Capabilities.Graph = true; p.Infra.Graph.Driver = "none" }, wantErr: "infra.graph.driver"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(p)
			err := p.Validate()
			require.Error(t, err, "expected validation error")
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.yaml")
	p, err := Load(path)
	require.Error(t, err)
	require.Nil(t, p)
	require.ErrorContains(t, err, path)
}

func TestLoadMalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("profile: [personal"), 0o600))
	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, path)
}

func validProfile() *Profile {
	return &Profile{
		Profile: "personal",
		Capabilities: Capabilities{
			Knowledge: true,
			Chat:      true,
			Search:    true,
		},
		Infra: Infra{
			DB:        DriverConfig{Driver: "postgres", DSN: "postgres://corekg:corekg@127.0.0.1:5432/corekg?sslmode=disable"},
			Vector:    DriverConfig{Driver: "pgvector"},
			Keyword:   DriverConfig{Driver: "postgres_fts"},
			Object:    DriverConfig{Driver: "localdir", Root: "./data/objects"},
			Cache:     DriverConfig{Driver: "memory"},
			Lock:      DriverConfig{Driver: "db"},
			Stream:    DriverConfig{Driver: "memory"},
			Queue:     DriverConfig{Driver: "db"},
			Embedding: DriverConfig{Driver: "http", URL: "http://127.0.0.1:11434/v1/embeddings"},
			Rerank:    DriverConfig{Driver: "none"},
			LLM:       DriverConfig{Driver: "http", URL: "http://127.0.0.1:11434/v1"},
			Parser:    DriverConfig{Driver: "local"},
			Graph:     DriverConfig{Driver: "none"},
		},
	}
}
