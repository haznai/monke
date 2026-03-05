default:
  just --list

start:
  go run -ldflags="-s -w" .
      



