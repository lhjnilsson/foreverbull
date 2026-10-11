package trading

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"gopkg.in/yaml.v3"
)

const defaultProfileCurrency = "USD"

// Registry is the set of trading profiles loaded from TRADING_PROFILES_FILE.
// Get returns the same client for a name for as long as the process runs.
type Registry struct {
	clients map[string]supplier.Trading
}

var _ supplier.Registry = (*Registry)(nil)

// RegistryOptions is supplied by module.go. NewRegistry does not read the environment.
type RegistryOptions struct {
	// Directory resolves a ticker alias to an ISIN. Nil fails closed on PlaceOrder.
	Directory Directory
	// HTTPClient is used for every profile. Nil lets each adapter build its own client.
	HTTPClient *http.Client
	// ListingsFile is used when a profile does not set listingsPath.
	ListingsFile string
}

type profileFile struct {
	Profiles []profileSpec `yaml:"profiles"`
}

type profileSpec struct {
	Name         string `yaml:"name"`
	Provider     string `yaml:"provider"`
	Currency     string `yaml:"currency"`
	APIKey       string `yaml:"apiKey"`
	APISecret    string `yaml:"apiSecret"`
	AccessToken  string `yaml:"accessToken"`
	AccountKey   string `yaml:"accountKey"`
	Identifier   string `yaml:"identifier"`
	Password     string `yaml:"password"`
	AccountID    string `yaml:"accountId"`
	AccountMode  string `yaml:"accountMode"`
	ListingsPath string `yaml:"listingsPath"`
}

// NewRegistry loads path when enabled is true.
// enabled false returns an empty registry and does not read path, so a missing
// file still starts the process. enabled true returns an error when path is
// empty, missing, or invalid. Paper and demo base URLs are the adapter constants.
func NewRegistry(path string, enabled bool, opts RegistryOptions) (*Registry, error) {
	if !enabled {
		return &Registry{}, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("trading profiles file is required when TRADING_ENABLED is true")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("trading profiles file: %w", err)
	}
	specs, err := parseProfiles(raw)
	if err != nil {
		return nil, err
	}
	return buildRegistry(specs, opts)
}

// Get returns the client built for name. An unknown name returns false.
func (r *Registry) Get(name string) (supplier.Trading, bool) {
	if r == nil {
		return nil, false
	}
	client, ok := r.clients[strings.TrimSpace(name)]
	if !ok || client == nil {
		return nil, false
	}
	return client, true
}

func parseProfiles(raw []byte) ([]profileSpec, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("trading profiles: %w", err)
	}
	if err := rejectBaseURL(&root); err != nil {
		return nil, err
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var doc profileFile
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("trading profiles: file is empty")
		}
		return nil, fmt.Errorf("trading profiles: %w", err)
	}

	seen := make(map[string]struct{}, len(doc.Profiles))
	out := make([]profileSpec, 0, len(doc.Profiles))
	for _, spec := range doc.Profiles {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, errors.New("trading profiles: name is empty")
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("trading profiles: duplicate name %q", name)
		}
		seen[name] = struct{}{}

		provider := strings.ToLower(strings.TrimSpace(spec.Provider))
		switch provider {
		case "alpaca", "saxo", "ibkr", "ig":
		default:
			return nil, fmt.Errorf("trading profiles: unknown provider %q", strings.TrimSpace(spec.Provider))
		}

		switch strings.ToLower(strings.TrimSpace(spec.AccountMode)) {
		case "", "paper":
		case "live":
			return nil, errors.New("trading profiles: live account mode is not allowed")
		default:
			return nil, fmt.Errorf("trading profiles: account mode %q", strings.TrimSpace(spec.AccountMode))
		}

		spec.Name = name
		spec.Provider = provider
		spec.Currency = strings.ToUpper(strings.TrimSpace(spec.Currency))
		if spec.Currency == "" {
			spec.Currency = defaultProfileCurrency
		}
		out = append(out, spec)
	}
	return out, nil
}

func rejectBaseURL(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if err := rejectBaseURL(child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if key == "base_url" || strings.EqualFold(key, "baseUrl") {
				return errors.New("trading profiles: baseUrl is not allowed")
			}
			if err := rejectBaseURL(node.Content[i+1]); err != nil {
				return err
			}
		}
	case yaml.AliasNode:
		if node.Alias != nil && node.Alias != node {
			return rejectBaseURL(node.Alias)
		}
	case yaml.ScalarNode:
	}
	return nil
}

func buildRegistry(specs []profileSpec, opts RegistryOptions) (*Registry, error) {
	clients := make(map[string]supplier.Trading, len(specs))
	for _, spec := range specs {
		client, err := New(spec.config(opts))
		if err != nil {
			return nil, fmt.Errorf("trading profile %q: %w", spec.Name, err)
		}
		clients[spec.Name] = client
	}
	return &Registry{clients: clients}, nil
}

func (spec profileSpec) config(opts RegistryOptions) Config {
	listings := strings.TrimSpace(spec.ListingsPath)
	if listings == "" {
		listings = opts.ListingsFile
	}
	cfg := Config{
		Provider:     spec.Provider,
		Currency:     spec.Currency,
		AllowLiveURL: false,
		ListingsFile: listings,
		HTTPClient:   opts.HTTPClient,
		Directory:    opts.Directory,
	}
	switch spec.Provider {
	case "alpaca":
		cfg.AlpacaAPIKey = spec.APIKey
		cfg.AlpacaAPISecret = spec.APISecret
	case "saxo":
		cfg.SaxoAccessToken = spec.AccessToken
		cfg.SaxoAccountKey = spec.AccountKey
	case "ig":
		cfg.IGAPIKey = spec.APIKey
		cfg.IGIdentifier = spec.Identifier
		cfg.IGPassword = spec.Password
	case "ibkr":
		cfg.IBKRAccountID = strings.TrimSpace(spec.AccountID)
		cfg.IBKRAccountMode = "paper"
		cfg.IBKRAccessToken = spec.AccessToken
	}
	return cfg
}
