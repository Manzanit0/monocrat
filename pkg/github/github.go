package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Manzanit0/go-github/v52/github"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/manzanit0/monocrat/pkg/httpx"
)

const (
	ApprovedDeploymentState = "approved"
	RejectedDeploymentState = "rejected"
)

// DeploymentProtectionRuleEvent
// https://docs.github.com/webhooks-and-events/webhooks/webhook-events-and-payloads#deployment_protection_rule
type DeploymentProtectionRuleEvent struct {
	Action                string `json:"action"`
	Environment           string `json:"environment"`
	Event                 string `json:"event"`
	DeploymentCallbackURL string `json:"deployment_callback_url"`
	Deployment            struct {
		URL                 string      `json:"url"`
		ID                  int         `json:"id"`
		NodeID              string      `json:"node_id"`
		Task                string      `json:"task"`
		OriginalEnvironment string      `json:"original_environment"`
		Environment         string      `json:"environment"`
		Description         interface{} `json:"description"`
		CreatedAt           time.Time   `json:"created_at"`
		UpdatedAt           time.Time   `json:"updated_at"`
		StatusesURL         string      `json:"statuses_url"`
		RepositoryURL       string      `json:"repository_url"`
		Creator             struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			NodeID            string `json:"node_id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"creator"`
		Sha                   string   `json:"sha"`
		Ref                   string   `json:"ref"`
		Payload               struct{} `json:"payload"`
		TransientEnvironment  bool     `json:"transient_environment"`
		ProductionEnvironment bool     `json:"production_environment"`
		PerformedViaGithubApp struct {
			ID     int    `json:"id"`
			Slug   string `json:"slug"`
			NodeID string `json:"node_id"`
			Owner  struct {
				Login             string `json:"login"`
				ID                int    `json:"id"`
				NodeID            string `json:"node_id"`
				AvatarURL         string `json:"avatar_url"`
				GravatarID        string `json:"gravatar_id"`
				URL               string `json:"url"`
				HTMLURL           string `json:"html_url"`
				FollowersURL      string `json:"followers_url"`
				FollowingURL      string `json:"following_url"`
				GistsURL          string `json:"gists_url"`
				StarredURL        string `json:"starred_url"`
				SubscriptionsURL  string `json:"subscriptions_url"`
				OrganizationsURL  string `json:"organizations_url"`
				ReposURL          string `json:"repos_url"`
				EventsURL         string `json:"events_url"`
				ReceivedEventsURL string `json:"received_events_url"`
				Type              string `json:"type"`
				SiteAdmin         bool   `json:"site_admin"`
			} `json:"owner"`
			Name        string    `json:"name"`
			Description string    `json:"description"`
			ExternalURL string    `json:"external_url"`
			HTMLURL     string    `json:"html_url"`
			CreatedAt   time.Time `json:"created_at"`
			UpdatedAt   time.Time `json:"updated_at"`
			Permissions struct {
				Actions             string `json:"actions"`
				Administration      string `json:"administration"`
				Checks              string `json:"checks"`
				Contents            string `json:"contents"`
				Deployments         string `json:"deployments"`
				Discussions         string `json:"discussions"`
				Issues              string `json:"issues"`
				MergeQueues         string `json:"merge_queues"`
				Metadata            string `json:"metadata"`
				Packages            string `json:"packages"`
				Pages               string `json:"pages"`
				PullRequests        string `json:"pull_requests"`
				RepositoryHooks     string `json:"repository_hooks"`
				RepositoryProjects  string `json:"repository_projects"`
				SecurityEvents      string `json:"security_events"`
				Statuses            string `json:"statuses"`
				VulnerabilityAlerts string `json:"vulnerability_alerts"`
			} `json:"permissions"`
			Events []string `json:"events"`
		} `json:"performed_via_github_app"`
	} `json:"deployment"`
	PullRequests []interface{} `json:"pull_requests"`
	Repository   struct {
		ID       int    `json:"id"`
		NodeID   string `json:"node_id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
		Owner    struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			NodeID            string `json:"node_id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"owner"`
		HTMLURL                  string        `json:"html_url"`
		Description              interface{}   `json:"description"`
		Fork                     bool          `json:"fork"`
		URL                      string        `json:"url"`
		ForksURL                 string        `json:"forks_url"`
		KeysURL                  string        `json:"keys_url"`
		CollaboratorsURL         string        `json:"collaborators_url"`
		TeamsURL                 string        `json:"teams_url"`
		HooksURL                 string        `json:"hooks_url"`
		IssueEventsURL           string        `json:"issue_events_url"`
		EventsURL                string        `json:"events_url"`
		AssigneesURL             string        `json:"assignees_url"`
		BranchesURL              string        `json:"branches_url"`
		TagsURL                  string        `json:"tags_url"`
		BlobsURL                 string        `json:"blobs_url"`
		GitTagsURL               string        `json:"git_tags_url"`
		GitRefsURL               string        `json:"git_refs_url"`
		TreesURL                 string        `json:"trees_url"`
		StatusesURL              string        `json:"statuses_url"`
		LanguagesURL             string        `json:"languages_url"`
		StargazersURL            string        `json:"stargazers_url"`
		ContributorsURL          string        `json:"contributors_url"`
		SubscribersURL           string        `json:"subscribers_url"`
		SubscriptionURL          string        `json:"subscription_url"`
		CommitsURL               string        `json:"commits_url"`
		GitCommitsURL            string        `json:"git_commits_url"`
		CommentsURL              string        `json:"comments_url"`
		IssueCommentURL          string        `json:"issue_comment_url"`
		ContentsURL              string        `json:"contents_url"`
		CompareURL               string        `json:"compare_url"`
		MergesURL                string        `json:"merges_url"`
		ArchiveURL               string        `json:"archive_url"`
		DownloadsURL             string        `json:"downloads_url"`
		IssuesURL                string        `json:"issues_url"`
		PullsURL                 string        `json:"pulls_url"`
		MilestonesURL            string        `json:"milestones_url"`
		NotificationsURL         string        `json:"notifications_url"`
		LabelsURL                string        `json:"labels_url"`
		ReleasesURL              string        `json:"releases_url"`
		DeploymentsURL           string        `json:"deployments_url"`
		CreatedAt                time.Time     `json:"created_at"`
		UpdatedAt                time.Time     `json:"updated_at"`
		PushedAt                 time.Time     `json:"pushed_at"`
		GitURL                   string        `json:"git_url"`
		SSHURL                   string        `json:"ssh_url"`
		CloneURL                 string        `json:"clone_url"`
		SvnURL                   string        `json:"svn_url"`
		Homepage                 interface{}   `json:"homepage"`
		Size                     int           `json:"size"`
		StargazersCount          int           `json:"stargazers_count"`
		WatchersCount            int           `json:"watchers_count"`
		Language                 interface{}   `json:"language"`
		HasIssues                bool          `json:"has_issues"`
		HasProjects              bool          `json:"has_projects"`
		HasDownloads             bool          `json:"has_downloads"`
		HasWiki                  bool          `json:"has_wiki"`
		HasPages                 bool          `json:"has_pages"`
		HasDiscussions           bool          `json:"has_discussions"`
		ForksCount               int           `json:"forks_count"`
		MirrorURL                interface{}   `json:"mirror_url"`
		Archived                 bool          `json:"archived"`
		Disabled                 bool          `json:"disabled"`
		OpenIssuesCount          int           `json:"open_issues_count"`
		License                  interface{}   `json:"license"`
		AllowForking             bool          `json:"allow_forking"`
		IsTemplate               bool          `json:"is_template"`
		WebCommitSignoffRequired bool          `json:"web_commit_signoff_required"`
		Topics                   []interface{} `json:"topics"`
		Visibility               string        `json:"visibility"`
		Forks                    int           `json:"forks"`
		OpenIssues               int           `json:"open_issues"`
		Watchers                 int           `json:"watchers"`
		DefaultBranch            string        `json:"default_branch"`
	} `json:"repository"`
	Sender struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"sender"`
	Installation struct {
		ID     int64  `json:"id"`
		NodeID string `json:"node_id"`
	} `json:"installation"`
}

type Client interface {
	ApproveDeployment(ctx context.Context, event *DeploymentProtectionRuleEvent) error
	RejectDeployment(ctx context.Context, event *DeploymentProtectionRuleEvent) error
}

type client struct {
	g          *github.Client
	owner      string
	repository string
}

type ErrorResponse struct {
	Message          string `json:"message"`
	Errors           string `json:"errors"`
	DocumentationURL string `json:"documentation_url"`
}

func NewClient(owner, repository string, appID int64, installationID int64, privateKey []byte) (Client, error) {
	// FIXME: this bit can be reused across installations.
	tr := httpx.NewLoggingRoundTripper()
	itr, err := ghinstallation.NewAppsTransport(tr, appID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("create transport from private key: %w", err)
	}

	c := github.NewClient(&http.Client{Transport: ghinstallation.NewFromAppsTransport(itr, installationID)})
	return &client{repository: repository, owner: owner, g: c}, nil
}

func (c *client) ApproveDeployment(ctx context.Context, event *DeploymentProtectionRuleEvent) error {
	return c.reviewDeployment(ctx, event, ApprovedDeploymentState)
}

func (c *client) RejectDeployment(ctx context.Context, event *DeploymentProtectionRuleEvent) error {
	return c.reviewDeployment(ctx, event, RejectedDeploymentState)
}

func (c *client) reviewDeployment(ctx context.Context, event *DeploymentProtectionRuleEvent, state string) error {
	runID, err := extractRunID(event.DeploymentCallbackURL)
	if err != nil {
		return fmt.Errorf("extracting run ID from event: %w", err)
	}

	log.Println("[info] requesting review for environment", event.Deployment.Environment, "and workflow run", runID)

	res, err := c.g.Actions.ReviewDeploymentProtectionRule(ctx, c.owner, c.repository, runID, &github.ReviewDeploymentProtectionRuleRequest{
		State:           state,
		Comment:         "signed-off by Monocrat",
		EnvironmentName: event.Deployment.Environment,
	})

	if err != nil {
		var errResp ErrorResponse
		dec := json.NewDecoder(res.Body)
		if err := dec.Decode(&errResp); err != nil && err != io.EOF {
			log.Println("[error] unmarshal body:", err.Error())
			return fmt.Errorf("review deployment: request failed + failed to parse body")
		}

		return fmt.Errorf("review deployment: %s: %s", errResp.Message, errResp.Errors)
	}

	return nil
}

func extractRunID(callbackURL string) (int64, error) {
	// Example URL:
	// https://api.github.com/repos/Manzanit0/gitops-env-per-folder-poc/actions/runs/4810948216/deployment_protection_rule
	s := strings.Split(callbackURL, "runs/")
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid callbackURL")
	}

	s2 := strings.Split(s[1], "/")
	if len(s2) < 1 {
		return 0, fmt.Errorf("invalid callbackURL")
	}

	runID, err := strconv.ParseInt(s2[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int: %w", err)
	}

	return runID, nil
}
