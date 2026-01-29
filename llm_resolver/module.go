package llm_resolver

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(LLMResolver{})
	httpcaddyfile.RegisterHandlerDirective("llm_resolver", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("llm_resolver", httpcaddyfile.Before, "reverse_proxy")
}

// LLMResolver is a Caddy HTTP handler module that resolves hostnames
// to upstream targets using an LLM (via OpenRouter API).
type LLMResolver struct {
	// APIKey is the API key for the LLM API
	APIKey string `json:"api_key,omitempty"`

	// APIURL is the URL for the LLM API (default: https://openrouter.ai/api/v1/chat/completions)
	APIURL string `json:"api_url,omitempty"`

	// Model is the LLM model to use (default: anthropic/claude-haiku-4.5)
	Model string `json:"model,omitempty"`

	// CacheFile is the path to store hostname mappings (default: /data/mappings.json)
	CacheFile string `json:"cache_file,omitempty"`

	// ComposeProject is the name of our own compose project to filter out
	ComposeProject string `json:"compose_project,omitempty"`

	// logger is the Caddy logger
	logger *zap.Logger

	// cache is the mapping cache
	cache *Cache

	// resolver handles LLM API calls
	resolver *Resolver
}

// CaddyModule returns the Caddy module information.
func (LLMResolver) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.llm_resolver",
		New: func() caddy.Module { return new(LLMResolver) },
	}
}

// Provision sets up the module.
func (m *LLMResolver) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger()

	// Set defaults
	if m.Model == "" {
		m.Model = "anthropic/claude-haiku-4.5"
	}
	if m.CacheFile == "" {
		m.CacheFile = "/data/mappings.json"
	}

	// Initialize cache
	m.cache = NewCache(m.CacheFile, m.logger)
	if err := m.cache.Load(); err != nil {
		m.logger.Warn("failed to load cache, starting fresh", zap.Error(err))
	}

	// Initialize resolver
	m.resolver = NewResolver(m.APIKey, m.APIURL, m.Model, m.ComposeProject, m.logger)

	m.logger.Info("LLM resolver provisioned",
		zap.String("model", m.Model),
		zap.String("cache_file", m.CacheFile),
	)

	return nil
}

// Validate validates the module configuration.
func (m *LLMResolver) Validate() error {
	// API key can be empty - we'll check at runtime
	return nil
}

// Cleanup is called when the module is being unloaded.
func (m *LLMResolver) Cleanup() error {
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *LLMResolver) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "api_key":
				if !d.NextArg() {
					return d.ArgErr()
				}
				m.APIKey = d.Val()
			case "api_url":
				if !d.NextArg() {
					return d.ArgErr()
				}
				m.APIURL = d.Val()
			case "model":
				if !d.NextArg() {
					return d.ArgErr()
				}
				m.Model = d.Val()
			case "cache_file":
				if !d.NextArg() {
					return d.ArgErr()
				}
				m.CacheFile = d.Val()
			case "compose_project":
				if !d.NextArg() {
					return d.ArgErr()
				}
				m.ComposeProject = d.Val()
			default:
				return d.Errf("unknown subdirective '%s'", d.Val())
			}
		}
	}
	return nil
}

// parseCaddyfile sets up the handler from Caddyfile tokens.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m LLMResolver
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return &m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*LLMResolver)(nil)
	_ caddy.Validator             = (*LLMResolver)(nil)
	_ caddy.CleanerUpper          = (*LLMResolver)(nil)
	_ caddyhttp.MiddlewareHandler = (*LLMResolver)(nil)
	_ caddyfile.Unmarshaler       = (*LLMResolver)(nil)
)
