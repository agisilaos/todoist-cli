# Repository-owned settings, including native Keychain CGO support.
CLI_NAME=todoist
FORMULA_NAME=todoist-cli
ARTIFACT_NAME=todoist-cli
DEFAULT_BRANCH=main
DEFAULT_HOMEBREW_DESC='Agentic CLI for Todoist'
DEFAULT_HOMEBREW_LICENSE=MIT
DEFAULT_HOMEBREW_TEST_ARG=--version
DEFAULT_FORMULA_PATH=Formula/todoist-cli.rb
DEFAULT_BUILD_PKG=./cmd/todoist
RELEASE_LDFLAGS_TEMPLATE='-s -w -X github.com/agisilaos/todoist-cli/internal/cli.Version={{VERSION}} -X github.com/agisilaos/todoist-cli/internal/cli.Commit={{COMMIT}} -X github.com/agisilaos/todoist-cli/internal/cli.Date={{DATE}}'
RELEASE_VERSION_TEMPLATE='todoist {{VERSION}} ({{COMMIT}}) {{DATE}}'
RELEASE_CGO_ENABLED=1
RELEASE_INCLUDE_LICENSE=1
CLI_TEMPLATE_FINGERPRINT=816f217b3a5c95477b24e3fb8ba1f3db5470654441b71b16e5385c9cb5b73290
RELEASE_GO_TOOLCHAIN="go1.27.1"
