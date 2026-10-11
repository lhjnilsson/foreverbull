package trading

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

const tradingHTTPTimeout = 30 * time.Second

// Config selects one broker adapter. New does not read the environment.
// Profile loading passes paper and demo URLs by leaving the base URL fields empty.
type Config struct {
	// Provider is alpaca, saxo, ibkr, or ig. Anything else is an error.
	Provider string
	// Currency is compared with the account currency. Empty means USD.
	Currency string
	// AllowLiveURL permits known live endpoints and IBKR account mode live.
	// It does not unlock deploy mode LIVE.
	AllowLiveURL bool
	// ListingsFile replaces the embedded sample when set.
	ListingsFile string
	// HTTPClient is used for broker calls. Nil gets a client with a 30s timeout.
	HTTPClient *http.Client
	// Directory resolves a ticker alias to an ISIN. Nil fails closed.
	Directory Directory

	AlpacaBaseURL   string
	AlpacaAPIKey    string
	AlpacaAPISecret string

	SaxoBaseURL     string
	SaxoAccessToken string
	SaxoAccountKey  string

	IBKRBaseURL     string
	IBKRAccountID   string
	IBKRAccountMode string
	IBKRAccessToken string

	IGBaseURL    string
	IGAPIKey     string
	IGIdentifier string
	IGPassword   string
}

// New builds the provider adapter and wraps it so an empty ISIN is resolved
// before the adapter sees the order. Live hosts and IBKR account mode live are
// refused unless AllowLiveURL is set. An empty IBKR account mode is paper.
func New(cfg Config) (supplier.Trading, error) {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: tradingHTTPTimeout}
	}
	book, err := Load(cfg.ListingsFile)
	if err != nil {
		return nil, err
	}

	mode := strings.TrimSpace(cfg.IBKRAccountMode)
	if mode == "" {
		mode = "paper"
	}

	var inner supplier.Trading
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "alpaca":
		inner, err = NewAlpaca(AlpacaConfig{
			BaseURL:      cfg.AlpacaBaseURL,
			APIKey:       cfg.AlpacaAPIKey,
			APISecret:    cfg.AlpacaAPISecret,
			Currency:     cfg.Currency,
			AllowLiveURL: cfg.AllowLiveURL,
		}, book, client)
	case "saxo":
		inner, err = NewSaxo(SaxoConfig{
			BaseURL:      cfg.SaxoBaseURL,
			AccessToken:  cfg.SaxoAccessToken,
			AccountKey:   cfg.SaxoAccountKey,
			Currency:     cfg.Currency,
			AllowLiveURL: cfg.AllowLiveURL,
		}, book, client)
	case "ibkr":
		inner, err = NewIBKR(IBKRConfig{
			BaseURL:      cfg.IBKRBaseURL,
			AccountID:    cfg.IBKRAccountID,
			AccountMode:  mode,
			AccessToken:  cfg.IBKRAccessToken,
			Currency:     cfg.Currency,
			AllowLiveURL: cfg.AllowLiveURL,
		}, book, client)
	case "ig":
		inner, err = NewIG(IGConfig{
			BaseURL:      cfg.IGBaseURL,
			APIKey:       cfg.IGAPIKey,
			Identifier:   cfg.IGIdentifier,
			Password:     cfg.IGPassword,
			Currency:     cfg.Currency,
			AllowLiveURL: cfg.AllowLiveURL,
		}, book, client)
	default:
		return nil, fmt.Errorf("unknown trading provider: %s", cfg.Provider)
	}
	if err != nil {
		return nil, err
	}
	return NewWrapper(inner, cfg.Directory), nil
}
