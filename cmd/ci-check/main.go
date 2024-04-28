package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Manzanit0/go-github/v52/github"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	githubwrap "github.com/manzanit0/monocrat/pkg/github"
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

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// This is the endpoint registered under the GitHub App where we will get our
	// "check_run" payloads.
	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		// TODO: we need to secure the webhook
		// https://docs.github.com/en/webhooks-and-events/webhooks/securing-your-webhooks#validating-payloads-from-github
		// payload, err := github.ValidatePayload(r, []byte(os.Getenv("MONOCRAT_WEBHOOK_SECRET")))

		switch r.Header.Get("X-Github-Event") {
		case "check_suite":
			var event github.CheckSuiteEvent
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&event); err != nil && err != io.EOF {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] unmarshal body:", err.Error())
			}

			if event.GetAction() == "completed" {
				log.Println("Ignoring check_suite event: completed")
				break
			}

			gh, err := githubwrap.NewClient(event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), appID, event.GetInstallation().GetID(), privateKey)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] initialising GitHub client:", err)
				return
			}

			err = gh.CreateCheckRun(r.Context(), event)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] failing check run:", err)
				return
			}
		case "check_run":
			var event github.CheckRunEvent
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&event); err != nil && err != io.EOF {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] unmarshal body:", err.Error())
			}

			if event.GetAction() == "completed" {
				log.Println("Ignoring check_run event: completed")
				break
			}

			gh, err := githubwrap.NewClient(event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), appID, event.GetInstallation().GetID(), privateKey)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] initialising GitHub client:", err)
				return
			}

			err = gh.FailCheckRun(r.Context(), event)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] failing check run:", err)
				return
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
