# CI check

This application is fairly simple: every time a commit is pushed to the repository, the GH App will trigger a "check". When that check passes, it will then attach an action to it for deploying. Finally, if the action is triggered, a new check is created.

What it means to showcase is how to use the Checks API to do background work and chain actionable checks.
