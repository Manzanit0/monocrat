# deployment-protection-rule

## Implementation notes

### about google/github-go

Since `google/github-go` currently doesn't support the newest custom deployment
rule endpoints, I had to fork it and add what I needed there. Might be worth
checking back there at some point to see if it's there and ditch the fork.

### Alternative ways of interacting with deploys

`gh` CLI extension: `https://github.com/yuri-1987/gh-deploy`

```sh
gh deploy --env production --run-id 4881224728 --repo "Manzanit0/gitops-env-per-folder-poc" --reject
```
