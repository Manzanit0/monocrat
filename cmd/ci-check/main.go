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
	"github.com/manzanit0/monocrat/pkg/lint"
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
			lintCheckRun, res, err := gh.Checks.CreateCheckRun(r.Context(), event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), github.CreateCheckRunOptions{
				Name:    "Lint",
				HeadSHA: event.GetCheckSuite().GetHeadSHA(),
			})
			if err != nil {
				err = toErr(res)
				log.Println("[error]", err)
				w.WriteHeader(http.StatusInternalServerError)
				break outer
			}

			go func() {
				ctx := context.Background()

				repositoryDirectory, err := CloneAndCheckout(event.GetRepo().GetCloneURL(), event.CheckSuite.GetHeadCommit().GetSHA())
				defer func() {
					log.Println("Deleting temp dir")
					err = os.RemoveAll(repositoryDirectory)
					if err != nil {
						panic(err)
					}
				}()

				modules, _, err := FindGoModules(repositoryDirectory)
				if err != nil {
					log.Println("[error]", err)
					return
				}

				var issues []lint.Issue
				for _, modulePath := range modules {
					modulePath := filepath.Dir(modulePath)
					report, err := lint.Lint(ctx, modulePath)
					if err != nil {
						log.Println("[error]", err)
						_, res, err := gh.Checks.UpdateCheckRun(ctx,
							event.GetRepo().GetOwner().GetLogin(),
							event.GetRepo().GetName(),
							lintCheckRun.GetID(),
							github.UpdateCheckRunOptions{
								Name:       "Lint",
								Status:     github.String("completed"),
								Conclusion: github.String("failure"),
								Output: &github.CheckRunOutput{
									Title:   github.String("Failed to run linters"),
									Summary: github.String(fmt.Sprintf("failed to run linters: %s", err.Error())),
								},
							})
						if err != nil {
							err = toErr(res)
							log.Println("[error]", err)
						}
						return
					}

					issues = append(issues, report.Issues...)
				}

				if len(issues) > 0 {
					var annotations []*github.CheckRunAnnotation
					for _, issue := range issues {
						if issue.Text == "" {
							continue
						}

						annotations = append(annotations, &github.CheckRunAnnotation{
							AnnotationLevel: github.String("error"),
							Title:           &issue.Text,
							Message:         github.String(fmt.Sprintf("%s (%s)", issue.Text, issue.FromLinter)),
							Path:            github.String(issue.Pos.Filename),
							StartLine:       github.Int(issue.Pos.Line),
							EndLine:         github.Int(issue.Pos.Line),
							StartColumn:     github.Int(issue.Pos.Offset),
							EndColumn:       github.Int(issue.Pos.Column),
						})
					}

					_, res, err := gh.Checks.UpdateCheckRun(ctx,
						event.GetRepo().GetOwner().GetLogin(),
						event.GetRepo().GetName(),
						lintCheckRun.GetID(),
						github.UpdateCheckRunOptions{
							Name:       "Lint",
							Status:     github.String("completed"),
							Conclusion: github.String("failure"),
							Output: &github.CheckRunOutput{
								Title:       github.String("Linter failed"),
								Summary:     github.String("Linter failed"),
								Annotations: annotations,
							},
						})
					if err != nil {
						err = toErr(res)
						log.Println("[error]", err)
					}
					return
				}

				_, res, err := gh.Checks.UpdateCheckRun(ctx,
					event.GetRepo().GetOwner().GetLogin(),
					event.GetRepo().GetName(),
					lintCheckRun.GetID(),
					github.UpdateCheckRunOptions{
						Name:       "Lint",
						Status:     github.String("completed"),
						Conclusion: github.String("success"),
						Actions: []*github.CheckRunAction{
							{
								Label:       "Release application",
								Description: "Build and push",
								Identifier:  "release_image",
							},
						},
					})
				if err != nil {
					err = toErr(res)
					log.Println("[error]", err)
				}
			}()

		case *github.CheckRunEvent:
			if event.GetAction() == "completed" {
				log.Println("Ignoring check_run event: completed")
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
						Status:  github.String("in_progress"),
					})
				if err != nil {
					err = toErr(res)
					log.Println("[error]", err)
					w.WriteHeader(http.StatusInternalServerError)
					break outer
				}

				go func() {
					err = BuildAndPushChangedApplications(
						context.Background(),
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
								Status:     github.String("completed"),
								Conclusion: github.String("failure"),
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
							Status:     github.String("completed"),
							Conclusion: github.String("success"),
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

func BuildAndPushChangedApplications(ctx context.Context, remote, beforeCommitSHA, afterCommitSHA, dockerHubUsername, dockerHubPassword string) error {
	repositoryPath, err := CloneAndCheckout(remote, beforeCommitSHA)
	if err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}

	defer func() {
		err = os.RemoveAll(repositoryPath)
		if err != nil {
			panic(err)
		}
	}()

	changedFiles, err := GetChangedFiles(repositoryPath, beforeCommitSHA, afterCommitSHA)
	if err != nil {
		return fmt.Errorf("get changed files: %w", err)
	}

	// Let's find All the Go modules and runnable applications in the
	// cloned repository.
	modules, applications, err := FindGoModules(repositoryPath)
	if err != nil {
		return fmt.Errorf("find Go modules and runnable apps: %w", err)
	}

	// Now that we have (1) changed files, (2) Go modules and (3) runnable
	// applications, we can just cross-check the data to find out all the
	// applications that need rebuilding based on if the changed file happened
	// in a module with runnable applications.
	// We'd want to rebuild all the apps in a module with a change.
	appsToRebuild, modulesToVendor := GetAppsToRebuild(changedFiles, modules, applications)

	// Let's vendor the dependencies because this will just get trickier inside
	// a container due to the private repositories.
	for modulePath := range modulesToVendor {
		err = VendorGoModule(ctx, modulePath)
		if err != nil {
			return fmt.Errorf("vendor module %s: %w", modulePath, err)
		}
	}

	// Now let's build images for all those nice apps and push them to Docker Hub.
	for app := range appsToRebuild {
		appName, appRelativeDirectory := GetAppNameAndDirectory(repositoryPath, app)
		log.Println("build and push", appName, appRelativeDirectory)

		err := image.BuildAndPush(ctx, &image.BuildAndPushOptions{
			DockerHubUsername:   dockerHubUsername,
			DockerHubPassword:   dockerHubPassword,
			DockerHubRepository: fmt.Sprintf("monocrat-%s", appName),
			RepositoryDirectory: repositoryPath,
			AppVersion:          "1.2.3",
			AppDirectory:        appRelativeDirectory,
		})
		if err != nil {
			return fmt.Errorf("build and push all things: %w", err)
		}
	}

	return nil
}

func CloneAndCheckout(remote, commit string) (string, error) {
	local, err := os.MkdirTemp("", "temp-repository")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}

	r, err := git.PlainClone(local, false, &git.CloneOptions{
		URL: remote,
	})
	if err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	})
	if err != nil {
		return "", fmt.Errorf("checkout: %w", err)
	}

	return local, nil
}

func GetChangedFiles(repositoryPath, beforeCommitSHA, afterCommitSHA string) ([]string, error) {
	r, err := git.PlainOpen(repositoryPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	beforeCommit, err := r.CommitObject(plumbing.NewHash(beforeCommitSHA))
	if err != nil {
		return nil, fmt.Errorf("get head commit: %w", err)
	}

	afterCommit, err := r.CommitObject(plumbing.NewHash(afterCommitSHA))
	if err != nil {
		return nil, fmt.Errorf("get merged commit: %w", err)
	}

	if beforeCommit.Hash.String() == afterCommit.Hash.String() {
		log.Println("same commit has HEAD; nothing to do")
		return []string{}, nil
	}

	patch, err := beforeCommit.Patch(afterCommit)
	if err != nil {
		return nil, fmt.Errorf("get git patch: %w", err)
	}

	var changedFiles []string
	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()

		switch {
		case to != nil:
			abs := filepath.Join(repositoryPath, to.Path())
			changedFiles = append(changedFiles, abs)
			log.Println("created/updated:", to.Path())

		case from != nil:
			abs := filepath.Join(repositoryPath, from.Path())
			changedFiles = append(changedFiles, abs)
			log.Println("deleted:", from.Path())

		default:
			return nil, fmt.Errorf("awkward change: %w", err)
		}
	}

	return changedFiles, nil
}

func FindGoModules(repositoryPath string) (modules []string, applications []string, err error) {
	err = filepath.WalkDir(repositoryPath, func(path string, d fs.DirEntry, err error) error {
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
	return
}

// VendorGoModule mod vendor for the module.
// modulePath should be of form "/foo/bar/baz", it being a directory, not the
// reference to the go.mod file.
func VendorGoModule(ctx context.Context, modulePath string) error {
	cmd := exec.CommandContext(ctx, "go", "mod", "vendor")
	cmd.Dir = modulePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s, %s", err.Error(), string(output))
	}

	return nil
}

// GetAppsToRebuild cross-checks the files changed, the available Go modules and
// applications and computes which modules to vendor and which applications to
// compile based on the changes.
//
// Modules and applications should be of the form "/foo/bar/go.mod" and
// "/foo/bar/main.go", thus being references to the actual files, not the
// directories.
func GetAppsToRebuild(changedFiles []string, modules []string, applications []string) (map[string]interface{}, map[string]interface{}) {
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
					}
				}
			}
		}
	}

	return appsToRebuild, modulesToVendor
}

// GetAppNameAndDirectory extracts the application's directory relative to the
// repository and the application's name. It assumes that the application's name
// is the directory just above the main.go file.
//
// Both repositoryPath and appPath are expected to be absolute paths, appPath
// being the path to the main.go file.
func GetAppNameAndDirectory(repositoryPath, appPath string) (string, string) {
	separator := fmt.Sprintf("%c", filepath.Separator)
	split := strings.Split(appPath, separator)
	appName := split[len(split)-2 : len(split)-1][0]
	appName = strings.ReplaceAll(appName, "_", "-")
	appRelativeDirectory := strings.Split(appPath, repositoryPath)[1]
	appRelativeDirectory = strings.TrimPrefix(appRelativeDirectory, separator)
	return appName, appRelativeDirectory
}
