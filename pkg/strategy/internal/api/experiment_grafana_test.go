package api

import (
	"net/url"
	"path"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
)

func TestGrafanaExecutionURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		grafanaURL   string
		executionID  string
		wantHostPath string
		wantID       string
	}{
		{
			name:         "kind default",
			grafanaURL:   environment.GrafanaURLDefault,
			executionID:  "exec-1",
			wantHostPath: "http://localhost:3000/d/ee9ep4jxtwkcgf",
			wantID:       "exec-1",
		},
		{
			name:         "trims origin slash",
			grafanaURL:   "http://localhost:3000/",
			executionID:  "abc-123",
			wantHostPath: "http://localhost:3000/d/ee9ep4jxtwkcgf",
			wantID:       "abc-123",
		},
		{
			name:         "remote origin",
			grafanaURL:   "https://grafana.example.com",
			executionID:  "run-9",
			wantHostPath: "https://grafana.example.com/d/ee9ep4jxtwkcgf",
			wantID:       "run-9",
		},
		{
			name:         "subpath install",
			grafanaURL:   "https://example.com/grafana",
			executionID:  "e1",
			wantHostPath: "https://example.com/grafana/d/ee9ep4jxtwkcgf",
			wantID:       "e1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GrafanaExecutionURL(tt.grafanaURL, tt.executionID)
			require.NoError(t, err)
			parsed, err := url.Parse(got)
			require.NoError(t, err)
			require.Equal(t, tt.wantHostPath, parsed.Scheme+"://"+parsed.Host+parsed.Path)
			require.Equal(t, tt.wantID, parsed.Query().Get("var-execution"))
			require.Equal(t, grafanaDashboardUID, path.Base(parsed.Path))
		})
	}
}

func TestGrafanaExecutionURLDefaultMatchesEnv(t *testing.T) {
	require.Equal(t, environment.GrafanaURLDefault, "http://localhost:3000")
	got, err := GrafanaExecutionURL(environment.GrafanaURLDefault, "uuid-1")
	require.NoError(t, err)
	require.Equal(t, "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=uuid-1", got)
}

func TestGrafanaExecutionURLRefusesAnInvalidOriginInsteadOfTheDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		grafanaURL string
		want       string
	}{
		{"not a url", "GRAFANA_URL must start with http:// or https://; set it to the Grafana origin, such as http://localhost:3000"},
		{"", "GRAFANA_URL is empty; set it to the Grafana origin, such as http://localhost:3000"},
		{"https://", "GRAFANA_URL has no host; set it to the Grafana origin, such as http://localhost:3000"},
	}
	for _, tt := range tests {
		t.Run(tt.grafanaURL, func(t *testing.T) {
			t.Parallel()
			got, err := GrafanaExecutionURL(tt.grafanaURL, "exec-2")
			require.EqualError(t, err, tt.want)
			require.Empty(t, got)
		})
	}
}
