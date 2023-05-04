package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Masterminds/vcs"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/manzanit0/monocrat/pkg/github"
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

	repositoryOwner := os.Getenv("REPOSITORY_OWNER")
	if len(repositoryOwner) == 0 {
		log.Fatal("[error] missing REPOSITORY_OWNER environment variable")
	}

	repositoryName := os.Getenv("REPOSITORY_NAME")
	if len(repositoryName) == 0 {
		log.Fatal("[error] missing REPOSITORY_NAME environment variable")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// This is the endpoint registered under the GitHub App where we will get our
	// "deployment_protection_rule" payloads.
	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		// TODO: we need to secure the webhook
		// https://docs.github.com/en/webhooks-and-events/webhooks/securing-your-webhooks#validating-payloads-from-github
		// payload, err := github.ValidatePayload(r, []byte(os.Getenv("MONOCRAT_WEBHOOK_SECRET")))

		var event github.DeploymentProtectionRuleEvent
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&event); err != nil && err != io.EOF {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("[error] unmarshal body:", err.Error())
		}

		remarshalled, err := json.Marshal(event)
		if err == nil {
			log.Println("[info] event received:", string(remarshalled))
		}

		// FIXME: This client should be reused per installations.
		gh, err := github.NewClient(repositoryOwner, repositoryName, appID, event.Installation.ID, privateKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("[error] initialising GitHub client:", err)
			return
		}

		remote := fmt.Sprintf("https://github.com/%s/%s", repositoryOwner, repositoryName)
		local, _ := ioutil.TempDir("", "go-vcs")
		repo, err := vcs.NewRepo(remote, local)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("[error] checking out repository:", err.Error())
			return
		}

		err = repo.Get()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("[error] checking out repository:", err.Error())
			return
		}

		commitInfo, err := repo.CommitInfo(event.Deployment.Sha)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("[error] checking commit info:", err.Error())
			return
		}

		// TODO: ideally what we want to do is check the
		// deployment_protection_rule against owners to approve or reject. The
		// below logic is merely to show how approving or rejecting would go in
		// an automated fashion.
		log.Println("[debug] commit message:", commitInfo.Message)
		if strings.Contains(commitInfo.Message, "approve") {
			err = gh.ApproveDeployment(r.Context(), &event)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] approving deployment:", err.Error())
				return
			}
		} else {
			err = gh.RejectDeployment(r.Context(), &event)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("[error] rejecting deployment:", err.Error())
				return
			}
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
