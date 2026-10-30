package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/services/ghws"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSPAFallbackHandler_GETServesIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(spaFallbackHandler([]byte("<html>index</html>")))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "<html>index</html>", w.Body.String())
}

func TestSPAFallbackHandler_NonGETReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(spaFallbackHandler([]byte("<html>index</html>")))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "route not found")
}

func TestWebSocketSubscriberConfigsForMultipleRepositories(t *testing.T) {
	vars := config.Vars{
		GitHubToken:   "token",
		GitHubHost:    "github.com",
		GitHubRepos:   "owner/one, owner/two",
		GitHubEvents:  "workflow_run, workflow_job",
		WebhookSecret: "secret",
	}

	configs := websocketSubscriberConfigs(vars)

	assert.Equal(t, []ghws.Config{
		{
			Token:  "token",
			Host:   "github.com",
			Repo:   "owner/one",
			Events: []string{"workflow_run", "workflow_job"},
			Secret: "secret",
		},
		{
			Token:  "token",
			Host:   "github.com",
			Repo:   "owner/two",
			Events: []string{"workflow_run", "workflow_job"},
			Secret: "secret",
		},
	}, configs)
}

func TestWebSocketSubscriberConfigsPreservesSingleRepositoryAndOrganization(t *testing.T) {
	base := config.Vars{
		GitHubToken:  "token",
		GitHubHost:   "github.com",
		GitHubEvents: "workflow_run",
	}

	repoVars := base
	repoVars.GitHubRepo = "owner/repo"
	assert.Equal(t, []ghws.Config{{
		Token:  "token",
		Host:   "github.com",
		Repo:   "owner/repo",
		Events: []string{"workflow_run"},
	}}, websocketSubscriberConfigs(repoVars))

	orgVars := base
	orgVars.GitHubOrg = "my-org"
	assert.Equal(t, []ghws.Config{{
		Token:  "token",
		Host:   "github.com",
		Org:    "my-org",
		Events: []string{"workflow_run"},
	}}, websocketSubscriberConfigs(orgVars))

	enterpriseVars := base
	enterpriseVars.GitHubEnterprise = "my-enterprise"
	assert.Equal(t, []ghws.Config{{
		Token:      "token",
		Host:       "github.com",
		Enterprise: "my-enterprise",
		Events:     []string{"workflow_run"},
	}}, websocketSubscriberConfigs(enterpriseVars))
}
