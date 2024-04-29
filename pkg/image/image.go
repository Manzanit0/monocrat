package image

import (
	"context"
	"fmt"
	"os"

	"dagger.io/dagger"
)

type BuildAndPushOptions struct {
	DockerHubUsername   string
	DockerHubPassword   string
	DockerHubRepository string
	RepositoryDirectory string
	AppVersion          string
	AppDirectory        string
}

// BuildAndPush builds the specified Go application and pushes the image to DockerHub.
func BuildAndPush(ctx context.Context, opts *BuildAndPushOptions) error {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return fmt.Errorf("dagger connect: %w", err)
	}
	defer client.Close()

	workspace := client.Host().Directory(opts.RepositoryDirectory)

	// Now let's build a multi-stage image
	builder := client.Container().
		From("golang:1.22").
		WithDirectory("/workspace", workspace).
		WithWorkdir("/workspace").
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOWORK", "off").
		WithEnvVariable("GOPRIVATE", "github.com/docker").
		WithExec([]string{"go", "build", "-ldflags", fmt.Sprintf("-X main.version=%s", opts.AppVersion), "-o", "app", opts.AppDirectory})

	prodImage := client.Container().
		From("alpine:3.19").
		WithFile("/bin/app", builder.File("/workspace/app")).
		WithEntrypoint([]string{"/bin/app"})

	// And push the image to DockerHub
	_, err = prodImage.
		WithRegistryAuth("docker.io", opts.DockerHubUsername, client.SetSecret("password", opts.DockerHubPassword)).
		Publish(ctx, fmt.Sprintf("%s/%s:%s", opts.DockerHubUsername, opts.DockerHubRepository, opts.AppVersion))
	if err != nil {
		return fmt.Errorf("build & publish image: %w", err)
	}

	return nil
}
