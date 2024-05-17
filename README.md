# Monocrat

Monocrat is a collection of GitHub application prototypes for different purposes. Currently there are two available:

- ci-check: Showcases how to use the Checks API to run linters and build docker images.
- deployment-protection-rule: Showcases how to extend the GitHub Deployments feature with [custom protection
  rules](https://docs.github.com/en/actions/deployment/protecting-deployments/creating-custom-deployment-protection-rules)

## Resources

- https://docs.github.com/en/apps/creating-github-apps/setting-up-a-github-app/creating-a-github-app
- https://trstringer.com/github-app-authentication/
- https://docs.github.com/en/webhooks-and-events/webhooks/webhook-events-and-payloads#deployment_protection_rule
- https://docs.github.com/en/rest/actions/workflow-runs#review-custom-deployment-protection-rules-for-a-workflow-run
- https://github.com/google/go-github/issues/2774

## Hacking

To build the image of a service:

```sh
docker build -t monocrat:latest --build-arg SERVICE_PATH=./cmd/deployment-protection-rule .
```
