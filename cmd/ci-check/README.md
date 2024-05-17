# CI check

GitHub Application which leverages the Checks API to lint, build and push
images.

This is just intended as a working prototype to validate how the GitHub APIs
would work.

## Implementation notes

### Using golangci-lint programatically

Ideally I'd be able to use the packages exported by the linter, but one of the dependencies of the `linter` module is in an `internal` directory, which makes it impossible to reuse, namely [pkgcache](https://github.com/golangci/golangci-lint/blob/28b3813c887621934c04ed29c75d6dcfbba2271f/internal/pkgcache/pkgcache.go).

An alternative would be to fork it and move the package under `pkg`, but I'm unsure it's worth it. The ergonomics of the packages aren't great either, it looked somewhat like this:

```go
	log := logutils.NewStderrLog("my-logger")
	log.SetLevel(logutils.LogLevelDebug)

	cfg := config.NewDefault()
	cfg.Linters = config.Linters{EnableAll: true}

	env := goutil.NewEnv(log)
	fileCache := fsutils.NewFileCache()
	lineCache := fsutils.NewLineCache(fileCache)

	dbManager, err := lintersdb.NewManager(log, cfg, lintersdb.NewLinterBuilder(), lintersdb.NewPluginModuleBuilder(log), lintersdb.NewPluginGoBuilder(log))
	if err != nil {
		return nil, err
	}

	lintersToRun := dbManager.GetAllEnabledByDefaultLinters()
	log.Debugf("amount of linters: %d", len(lintersToRun))

	loadGuard := load.NewGuard()
	pkgLoader := lint.NewPackageLoader(log, cfg, []string{}, env, loadGuard)

	sw := timeutils.NewStopwatch("foo", log)
	cache, err := pkgcache.NewCache(sw, log) // <--- This is the package that can't be imported.
	if err != nil {
		return nil, err
	}

	contextBuilder := lint.NewContextBuilder(cfg, pkgLoader, fileCache, cache, loadGuard)
	lintCtx, err := contextBuilder.Build(context.TODO(), log, lintersToRun)
	if err != nil {
		return nil, fmt.Errorf("context loading failed: %w", err)
	}

	runner, err := lint.NewRunner(log, cfg, []string{}, env, lineCache, fileCache, dbManager, lintCtx)
	if err != nil {
		return nil, err
	}

	issues, err := runner.Run(context.TODO(), lintersToRun)
	if err != nil {
		return nil, err
	}

	return issues, nil
```
