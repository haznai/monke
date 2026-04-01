set dotenv-load

ldflags := "-s -w -X 'github.com/hazn/monkeytype-tui/internal/llm.embeddedAPIKey=" + env("GROQ_API_KEY", "") + "'"

default:
  just --list

start:
  go run -ldflags="{{ldflags}}" .

install:
  CGO_ENABLED=0 go build -trimpath -ldflags="{{ldflags}}" -o /opt/homebrew/bin/monkeytype-tui .
  ln -sf /opt/homebrew/bin/monkeytype-tui /opt/homebrew/bin/monke
  @echo "installed monke to /opt/homebrew/bin/"

test:
  go test ./... -v
