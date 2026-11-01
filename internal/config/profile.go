// Package config loads and validates deployment profile configuration for the
// corekg monolith.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile is the top-level deployment profile describing enabled capabilities
// and the infrastructure drivers backing them.
type Profile struct {
	Profile      string       `yaml:"profile"`
	Capabilities Capabilities `yaml:"capabilities"`
	Infra        Infra        `yaml:"infra"`
}

// Capabilities toggles the feature surface exposed by a deployment.
type Capabilities struct {
	Knowledge          bool `yaml:"knowledge"`
	Chat               bool `yaml:"chat"`
	Search             bool `yaml:"search"`
	Graph              bool `yaml:"graph"`
	OCR                bool `yaml:"ocr"`
	Video              bool `yaml:"video"`
	Writing            bool `yaml:"writing"`
	MCP                bool `yaml:"mcp"`
	Billing            bool `yaml:"billing"`
	ExternalDataSource bool `yaml:"external_datasource"`
}

// Infra selects the driver used by each infrastructure concern.
type Infra struct {
	DB        DriverConfig `yaml:"db"`
	Vector    DriverConfig `yaml:"vector"`
	Keyword   DriverConfig `yaml:"keyword"`
	Object    DriverConfig `yaml:"object"`
	Cache     DriverConfig `yaml:"cache"`
	Lock      DriverConfig `yaml:"lock"`
	Stream    DriverConfig `yaml:"stream"`
	Queue     DriverConfig `yaml:"queue"`
	Embedding DriverConfig `yaml:"embedding"`
	Rerank    DriverConfig `yaml:"rerank"`
	LLM       DriverConfig `yaml:"llm"`
	Parser    DriverConfig `yaml:"parser"`
	Graph     DriverConfig `yaml:"graph"`
}

// DriverConfig holds the driver name and its connection parameters.
type DriverConfig struct {
	Driver  string            `yaml:"driver"`
	DSN     string            `yaml:"dsn,omitempty"`
	URL     string            `yaml:"url,omitempty"`
	Addr    string            `yaml:"addr,omitempty"`
	Root    string            `yaml:"root,omitempty"`
	Bucket  string            `yaml:"bucket,omitempty"`
	Mode    string            `yaml:"mode,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

func (c DriverConfig) field(name string) string {
	switch name {
	case "dsn":
		return c.DSN
	case "url":
		return c.URL
	case "addr":
		return c.Addr
	case "root":
		return c.Root
	case "bucket":
		return c.Bucket
	}
	return ""
}

// Load reads and decodes the profile YAML file at path.
func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load profile %s: %w", path, err)
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("load profile %s: %w", path, err)
	}
	return &p, nil
}

// Validate checks the profile name, every infra driver, and the consistency
// between enabled capabilities and their required drivers. All violations are
// reported at once; each message contains the offending field path.
func (p *Profile) Validate() error {
	var errs []error

	switch p.Profile {
	case "personal", "aio", "enterprise":
	default:
		errs = append(errs, fmt.Errorf("profile: unsupported profile %q (allowed: personal|aio|enterprise)", p.Profile))
	}

	for _, rule := range p.driverRules() {
		if err := requireDriver(rule.section, rule.cfg.Driver, rule.allowed...); err != nil {
			errs = append(errs, err)
		}
		if err := rule.require(rule.cfg); err != nil {
			errs = append(errs, err)
		}
	}

	if p.Capabilities.Chat && p.Infra.LLM.Driver == "" {
		errs = append(errs, errors.New("capabilities.chat: requires infra.llm.driver to be set"))
	}
	if p.Capabilities.Search {
		if p.Infra.Vector.Driver == "" {
			errs = append(errs, errors.New("capabilities.search: requires infra.vector.driver to be set"))
		}
		if p.Infra.Embedding.Driver == "" {
			errs = append(errs, errors.New("capabilities.search: requires infra.embedding.driver to be set"))
		}
	}
	if p.Capabilities.Graph && p.Infra.Graph.Driver == "none" {
		errs = append(errs, errors.New(`capabilities.graph: requires infra.graph.driver other than "none"`))
	}

	return errors.Join(errs...)
}

type driverRule struct {
	section string
	cfg     DriverConfig
	allowed []string
	require func(cfg DriverConfig) error
}

func (p *Profile) driverRules() []driverRule {
	return []driverRule{
		{section: "db", cfg: p.Infra.DB, allowed: []string{"postgres"}, require: func(cfg DriverConfig) error {
			return requireValue("db.dsn", cfg.DSN, cfg.Driver)
		}},
		{section: "vector", cfg: p.Infra.Vector, allowed: []string{"pgvector", "elasticsearch", "milvus", "qdrant"}, require: func(cfg DriverConfig) error {
			switch cfg.Driver {
			case "elasticsearch", "milvus", "qdrant":
				if cfg.Addr == "" && cfg.URL == "" {
					return fmt.Errorf("infra.vector.addr: driver %s requires addr or url", cfg.Driver)
				}
			}
			return nil
		}},
		{section: "keyword", cfg: p.Infra.Keyword, allowed: []string{"postgres_fts", "elasticsearch"}, require: func(cfg DriverConfig) error {
			if cfg.Driver == "elasticsearch" {
				return requireValue("keyword.addr", cfg.Addr, cfg.Driver)
			}
			return nil
		}},
		{section: "object", cfg: p.Infra.Object, allowed: []string{"localdir", "minio", "s3"}, require: func(cfg DriverConfig) error {
			switch cfg.Driver {
			case "localdir":
				return requireValue("object.root", cfg.Root, cfg.Driver)
			case "minio", "s3":
				if err := requireValue("object.url", cfg.URL, cfg.Driver); err != nil {
					return err
				}
				return requireValue("object.bucket", cfg.Bucket, cfg.Driver)
			}
			return nil
		}},
		{section: "cache", cfg: p.Infra.Cache, allowed: []string{"memory", "redis"}, require: requireAddrWhen("cache", "redis")},
		{section: "lock", cfg: p.Infra.Lock, allowed: []string{"db", "redis"}, require: requireAddrWhen("lock", "redis")},
		{section: "stream", cfg: p.Infra.Stream, allowed: []string{"memory", "redis"}, require: requireAddrWhen("stream", "redis")},
		{section: "queue", cfg: p.Infra.Queue, allowed: []string{"db", "nats"}, require: requireValueWhen("queue", "nats", "url")},
		{section: "embedding", cfg: p.Infra.Embedding, allowed: []string{"http", "local"}, require: requireValueWhen("embedding", "http", "url")},
		{section: "rerank", cfg: p.Infra.Rerank, allowed: []string{"none", "http", "local"}, require: requireValueWhen("rerank", "http", "url")},
		{section: "llm", cfg: p.Infra.LLM, allowed: []string{"http", "openai"}, require: func(cfg DriverConfig) error {
			switch cfg.Driver {
			case "http", "openai":
				return requireValue("llm.url", cfg.URL, cfg.Driver)
			}
			return nil
		}},
		{section: "parser", cfg: p.Infra.Parser, allowed: []string{"local", "http"}, require: requireValueWhen("parser", "http", "url")},
		{section: "graph", cfg: p.Infra.Graph, allowed: []string{"none", "postgres", "nebulagraph"}, require: requireAddrWhen("graph", "nebulagraph")},
	}
}

func requireAddrWhen(section, driver string) func(cfg DriverConfig) error {
	return func(cfg DriverConfig) error {
		if cfg.Driver != driver {
			return nil
		}
		return requireValue(section+".addr", cfg.Addr, driver)
	}
}

func requireValueWhen(section, driver, field string) func(cfg DriverConfig) error {
	return func(cfg DriverConfig) error {
		if cfg.Driver != driver {
			return nil
		}
		return requireValue(section+"."+field, cfg.field(field), driver)
	}
}

// requireValue reports a missing required connection parameter, embedding the
// full field path in the message.
func requireValue(field, value, driver string) error {
	if value != "" {
		return nil
	}
	_, name, _ := strings.Cut(field, ".")
	return fmt.Errorf("infra.%s: driver %s requires %s to be set", field, driver, name)
}

// requireDriver reports an unsupported driver for the section, embedding the
// full field path in the message.
func requireDriver(section, driver string, allowed ...string) error {
	for _, a := range allowed {
		if driver == a {
			return nil
		}
	}
	return fmt.Errorf("infra.%s.driver: unsupported driver %q (allowed: %s)", section, driver, strings.Join(allowed, "|"))
}
