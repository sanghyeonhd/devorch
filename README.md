# devorch (Step1)

## Setup
```bash
export OPENAI_API_KEY="..."
export OPENROUTER_API_KEY="..."
export DEVORCH_DB_PATH="./devorch.db"
```

## Build
```bash
go mod tidy
go build ./...
```

## Doctor
```bash
./devorch doctor
```

## Providers
```bash
./devorch providers
```

## Models
```bash
./devorch models --provider openrouter
```

## Chat (records into okaon_runs)
```bash
./devorch chat --provider openrouter --model openai/gpt-4o-mini --prompt "hello"
# sqlite3 devorch.db "select provider, model, latency_ms, ok, quality from okaon_runs order by created_at desc limit 5;"
```
