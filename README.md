# gorun

`gorun` is a command to replace `go run` for live-reloading Go application.

# Install

```
go install github.com/defool/gorun@v1.0.0
```

# Usage

```
export GORUN_SKIP_DIRS=".env:node_packages"
gorun xxx.go
```
This command forks new process to run `go run xxx.go`, and recreate it after any `*.go` file is changed in current directory.
