package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Manzanit0/go-github/v52/github"
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/manzanit0/monocrat/pkg/httpx"
	"github.com/manzanit0/monocrat/pkg/image"
)

func main() {
	appID, err := strconv.ParseInt(os.Getenv("MONOCRAT_APP_ID"), 10, 64)
	if err != nil {
		log.Fatal("[error] parsing monocrat App ID:", err)
	}

	privateKey := []byte(os.Getenv("MONOCRAT_PRIVATE_KEY"))
	if len(privateKey) == 0 {
		log.Fatal("[error] missing MONOCRAT_PRIVATE_KEY environment variable")
	}

	dockerHubUsername := os.Getenv("DOCKER_HUB_USERNAME")
	if dockerHubUsername == "" {
		log.Fatal("[error] missing DOCKER_HUB_USERNAME environment variable")
	}

	dockerHubPassword := os.Getenv("DOCKER_HUB_PASSWORD")
	if dockerHubUsername == "" {
		log.Fatal("[error] missing DOCKER_HUB_PASSWORD environment variable")
	}

	tr := httpx.NewLoggingRoundTripper()
	itr, err := ghinstallation.NewAppsTransport(tr, appID, privateKey)
	if err != nil {
		log.Fatalf("[error] create transport from private key: %s", err.Error())
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		payload, err := github.ValidatePayload(r, []byte{})
		if err != nil {
			log.Println("[error] validate payload", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write([]byte(""))
			if err != nil {
				log.Println("[error] writing reponse:", err.Error())
				return
			}
			return
		}

		event, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			log.Println("[error] parse webhook", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write([]byte(""))
			if err != nil {
				log.Println("[error] writing reponse:", err.Error())
				return
			}
			return
		}

	outer:
		switch event := event.(type) {
		case *github.CheckSuiteEvent:
			if event.GetAction() == "completed" {
				log.Println("Ignoring check_suite event: completed")
				break outer
			}

			gh := github.NewClient(&http.Client{Transport: ghinstallation.NewFromAppsTransport(itr, event.GetInstallation().GetID())})
			_, res, err := gh.Checks.CreateCheckRun(r.Context(), event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), github.CreateCheckRunOptions{
				Name:    "Checking commit message",
				HeadSHA: event.GetCheckSuite().GetHeadSHA(),
			})
			if err != nil {
				err = toErr(res)
				log.Println("[error]", err)
				w.WriteHeader(http.StatusInternalServerError)
				break outer
			}

		case *github.CheckRunEvent:
			if event.GetAction() == "completed" {
				log.Println("Ignoring check_run event: completed")
				break outer
			}

			if event.GetAction() == "created" && event.GetCheckRun().GetName() == "Checking commit message" {
				gh := github.NewClient(&http.Client{Transport: ghinstallation.NewFromAppsTransport(itr, event.GetInstallation().GetID())})
				_, res, err := gh.Checks.UpdateCheckRun(r.Context(),
					event.GetRepo().GetOwner().GetLogin(),
					event.GetRepo().GetName(),
					event.CheckRun.GetID(),
					github.UpdateCheckRunOptions{
						Name:       event.GetCheckRun().GetName(),
						Status:     s("completed"),
						Conclusion: s("success"),
						Actions: []*github.CheckRunAction{
							{
								Label:       "Release application",
								Description: "Build and push image",
								Identifier:  "release_image",
							},
						},
					})
				if err != nil {
					err = toErr(res)
					log.Println("[error]", err)
					w.WriteHeader(http.StatusInternalServerError)
				}

				break outer
			}

			if event.GetAction() == "requested_action" {
				gh := github.NewClient(&http.Client{Transport: ghinstallation.NewFromAppsTransport(itr, event.GetInstallation().GetID())})
				releaseCheckRun, res, err := gh.Checks.CreateCheckRun(r.Context(),
					event.GetRepo().GetOwner().GetLogin(),
					event.GetRepo().GetName(),
					github.CreateCheckRunOptions{
						Name:    "Release application",
						HeadSHA: event.GetCheckRun().GetHeadSHA(),
						Status:  s("in_progress"),
					})
				if err != nil {
					err = toErr(res)
					log.Println("[error]", err)
					w.WriteHeader(http.StatusInternalServerError)
					break outer
				}

				go func() {
					err = BuildAndPushChangedApplications(
						event.GetRepo().GetCloneURL(),
						event.GetCheckRun().GetCheckSuite().GetBeforeSHA(),
						event.GetCheckRun().GetCheckSuite().GetAfterSHA(),
						dockerHubUsername,
						dockerHubPassword,
					)
					if err != nil {
						log.Println("[error]", err)
						_, res, err := gh.Checks.UpdateCheckRun(context.Background(),
							event.GetRepo().GetOwner().GetLogin(),
							event.GetRepo().GetName(),
							releaseCheckRun.GetID(),
							github.UpdateCheckRunOptions{
								Name:       "Release application",
								Status:     s("completed"),
								Conclusion: s("failure"),
								Actions: []*github.CheckRunAction{
									{
										Label:       "Retry release",
										Description: "Retry build and push",
										Identifier:  "release_image_retry",
									},
								},
							})
						if err != nil {
							err = toErr(res)
							log.Println("[error]", err)
							return
						}
					}

					_, res, err := gh.Checks.UpdateCheckRun(context.Background(),
						event.GetRepo().GetOwner().GetLogin(),
						event.GetRepo().GetName(),
						releaseCheckRun.GetID(),
						github.UpdateCheckRunOptions{
							Name:       "Release application",
							Status:     s("completed"),
							Conclusion: s("success"),
						})
					if err != nil {
						err = toErr(res)
						log.Println("[error]", err)
					}
				}()
				break outer
			}

		default:
			log.Println("Ignoring event: not a check_run or check_suite")
		}

		_, err = w.Write([]byte(""))
		if err != nil {
			log.Println("[error] writing reponse:", err.Error())
			return
		}
	})

	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}

	log.Println("[info] starting server on port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Println("[error] ListenAndServe", err)
	}
}

func toErr(res *github.Response) error {
	var errResp github.ErrorResponse
	dec := json.NewDecoder(res.Body)
	if err := dec.Decode(&errResp); err != nil && err != io.EOF {
		return fmt.Errorf("unmarshal body: %w", err)
	}

	return fmt.Errorf("%s: %s", errResp.Message, errResp.Errors)
}

func s(ss string) *string {
	return &ss
}

func BuildAndPushChangedApplications(remote, beforeCommitSHA, afterCommitSHA, dockerHubUsername, dockerHubPassword string) error {
	local, err := os.MkdirTemp("", "temp-repository")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}

	defer func() {
		err = os.RemoveAll(local)
		if err != nil {
			panic(err)
		}
	}()

	log.Println("Created directory", local)

	r, err := git.PlainClone(local, false, &git.CloneOptions{
		URL: remote,
	})
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	log.Println("Cloned", remote)

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	beforeCommit, err := r.CommitObject(plumbing.NewHash(beforeCommitSHA))
	if err != nil {
		return fmt.Errorf("get head commit: %w", err)
	}

	log.Printf("Got HEAD: %q\n", strings.Split(beforeCommit.Message, "\n")[0])

	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(afterCommitSHA),
	})
	if err != nil {
		return fmt.Errorf("checkout: %w", err)
	}

	log.Println("Checked out ", afterCommitSHA)

	afterCommit, err := r.CommitObject(plumbing.NewHash(afterCommitSHA))
	if err != nil {
		return fmt.Errorf("get merged commit: %w", err)
	}

	log.Printf("Got commit: %q\n", strings.Split(afterCommit.Message, "\n")[0])

	if beforeCommit.Hash.String() == afterCommit.Hash.String() {
		log.Println("same commit has HEAD; nothing to do")
		return nil
	}

	patch, err := beforeCommit.Patch(afterCommit)
	if err != nil {
		return fmt.Errorf("get git patch: %w", err)
	}

	var changedFiles []string
	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()

		switch {
		case to != nil:
			abs := filepath.Join(local, to.Path())
			changedFiles = append(changedFiles, abs)
			log.Println("created/updated:", to.Path())
		case from != nil:
			abs := filepath.Join(local, from.Path())
			changedFiles = append(changedFiles, abs)
			log.Println("deleted:", from.Path())
		default:
			log.Fatalln("Both to and from are nil")
		}
	}

	// Let's find All the Go modules and runnable applications in the
	// cloned repository.
	var modules []string
	var applications []string
	err = filepath.WalkDir(local, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("checking directory entry: %w", err)
		}

		if d.Name() == ".git" || d.Name() == ".github" {
			return filepath.SkipDir
		}

		if !d.IsDir() && d.Name() == "go.mod" {
			modules = append(modules, path)
			return nil
		}

		if !d.IsDir() && d.Name() == "main.go" {
			applications = append(applications, path)
			return nil
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("find Go modules and runnable apps: %w", err)
	}

	// Now that we have (1) changed files, (2) Go modules and (3) runnable
	// applications, we can just cross-check the data to find out all the
	// applications that need rebuilding based on if the changed file happened
	// in a module with runnable applications.
	// We'd want to rebuild all the apps in a module with a change.
	appsToRebuild := map[string]interface{}{}
	modulesToVendor := map[string]interface{}{}
	for _, module := range modules {
		moduleDir := filepath.Dir(module)
		for _, app := range applications {
			if strings.Contains(app, moduleDir) {
				for _, change := range changedFiles {
					if strings.Contains(change, moduleDir) {
						modulesToVendor[moduleDir] = nil
						appsToRebuild[app] = nil
						// log.Println("Needs rebuild:", app)
					}
				}
			}
		}
	}

	// Let's vendor the dependencies because this will just get trickier inside
	// a container due to the private repositories.
	for module := range modulesToVendor {
		cmd := exec.CommandContext(context.TODO(), "go", "mod", "vendor")
		cmd.Dir = module
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s, %s", err.Error(), string(output))
		}
	}

	// Now let's build images for all those nice apps and push them to Docker Hub.
	for app := range appsToRebuild {
		separator := fmt.Sprintf("%c", filepath.Separator)
		split := strings.Split(app, separator)
		appName := split[len(split)-2 : len(split)-1][0]
		appName = strings.ReplaceAll(appName, "_", "-")
		appRelativeDirectory := strings.Split(app, local)[1]
		appRelativeDirectory = strings.TrimPrefix(appRelativeDirectory, separator)

		log.Println("build and push", appName, appRelativeDirectory)
		err := image.BuildAndPush(context.TODO(), &image.BuildAndPushOptions{
			DockerHubUsername:   dockerHubUsername,
			DockerHubPassword:   dockerHubPassword,
			DockerHubRepository: fmt.Sprintf("monocrat-%s", appName),
			RepositoryDirectory: local,
			AppVersion:          "1.2.3",
			AppDirectory:        appRelativeDirectory,
		})
		if err != nil {
			return fmt.Errorf("build and push all things: %w", err)
		}
	}

	return nil
}
