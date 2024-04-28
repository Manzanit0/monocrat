package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Manzanit0/go-github/v52/github"
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/manzanit0/monocrat/pkg/httpx"
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

			if event.GetAction() == "requested_action" {
				gh := github.NewClient(&http.Client{Transport: ghinstallation.NewFromAppsTransport(itr, event.GetInstallation().GetID())})
				_, res, err := gh.Checks.CreateCheckRun(r.Context(),
					event.GetRepo().GetOwner().GetLogin(),
					event.GetRepo().GetName(),
					github.CreateCheckRunOptions{
						Name:       "Deploy to production",
						HeadSHA:    event.GetCheckRun().GetHeadSHA(),
						Conclusion: s("success"),
						Status:     s("completed"),
					})
				if err != nil {
					err = toErr(res)
					log.Println("[error]", err)
					w.WriteHeader(http.StatusInternalServerError)
					break outer
				}
				break outer
			}

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
							Label:       "Deploy to production",
							Description: "Deployes the application to production",
							Identifier:  "badibum",
						},
					},
				})
			if err != nil {
				err = toErr(res)
				log.Println("[error]", err)
				w.WriteHeader(http.StatusInternalServerError)
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
