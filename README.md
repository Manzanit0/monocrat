# Monocrat

Monocrat is a GitHub App for [custom protection
rules](https://docs.github.com/en/actions/deployment/protecting-deployments/creating-custom-deployment-protection-rules).

> **Note**
>
> Since `google/github-go` currently doesn't support the newest custom
> deployment rule endpoints, I had to fork it and add what I needed there. Might
> be worth checking back there at some point to see if it's there and ditch the
> fork.

## Alternative ways of interacting with deploys

`gh` CLI extension: `https://github.com/yuri-1987/gh-deploy`

```sh
gh deploy --env production --run-id 4881224728 --repo "Manzanit0/gitops-env-per-folder-poc" --reject
```

## Resources

- https://docs.github.com/en/apps/creating-github-apps/setting-up-a-github-app/creating-a-github-app
- https://trstringer.com/github-app-authentication/
- https://docs.github.com/en/webhooks-and-events/webhooks/webhook-events-and-payloads#deployment_protection_rule
- https://docs.github.com/en/rest/actions/workflow-runs#review-custom-deployment-protection-rules-for-a-workflow-run
- https://github.com/google/go-github/issues/2774
