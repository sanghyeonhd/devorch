# devorch

**Local-First Multi-LLM Orchestrator** - A comprehensive AI model orchestration system with OkAON (Outcome-KPI-Aware Orchestration Network) for intelligent model routing and learning.

## 🎯 Features

### Core Capabilities
- **Multi-Provider Support**: OpenAI, Anthropic, Google, OpenRouter, Ollama (local)
- **Intelligent Routing**: OkAON-based model selection with Thompson Sampling
- **Local-First**: Embedded SQLite database, no external dependencies
- **Cross-Platform**: macOS, Linux, Windows (amd64/arm64)
- **CGO-Free**: Pure Go build with `modernc.org/sqlite`

### Phase 1-40 Implementation Complete ✅

| Phase | Component | Description |
|-------|-----------|-------------|
| 1-5 | Core | CLI, providers, wire DI, storage |
| 6-10 | OkAON | Learning data lake, bench framework |
| 11-15 | Router | Policy, category routing, fallback chains |
| 16-20 | Quality | Evaluators, drift detection, experiments |
| 21-25 | Learning | Thompson sampling, UCB, reward calculation |
| 26-30 | Platform | Model resolver, delegate, background tasks |
| 31-35 | Extended | HW profile, repo fingerprint, composite eval |
| 36-40 | Advanced | Task classification, ensemble, A/B testing |

## 📦 Installation

### Prerequisites
- Go 1.22+ (recommended: Go 1.25+)

### Build from Source

```bash
# Clone and build
git clone https://github.com/your-repo/devorch.git
cd devorch
go mod tidy
go build -o bin/devorch ./cmd/devorch
```

### Cross-Platform Build

```bash
# macOS
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/devorch-darwin-amd64 ./cmd/devorch
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/devorch-darwin-arm64 ./cmd/devorch

# Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/devorch-linux-amd64 ./cmd/devorch
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/devorch-linux-arm64 ./cmd/devorch

# Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/devorch-windows-amd64.exe ./cmd/devorch
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/devorch-windows-arm64.exe ./cmd/devorch
```

## 🚀 Quick Start

### Environment Setup

```bash
# API Keys (optional - for cloud providers)
export OPENAI_API_KEY="sk-..."
export OPENROUTER_API_KEY="..."
export ANTHROPIC_API_KEY="..."
export GOOGLE_API_KEY="..."

# Configuration
export DEVORCH_DB_PATH="./devorch.db"      # SQLite database path
export DEVORCH_OFFLINE=1                    # Offline mode
export DEVORCH_AUTO_INSTALL=1               # Auto-install Ollama models
export DEVORCH_OLLAMA_HOST="http://127.0.0.1:11434"
```

### Basic Commands

```bash
# Health check
./devorch doctor

# List available providers
./devorch providers

# List models for a provider
./devorch models --provider ollama
./devorch models --provider openai
./devorch models --provider openrouter

# Pull Ollama model
./devorch ollama-pull --model llama3.2

# Chat (records to OkAON learning store)
./devorch chat --provider ollama --model llama3.2 --prompt "Hello, world!"
./devorch chat --provider openrouter --model openai/gpt-4o-mini --prompt "Explain Go interfaces"
```

## 🏗️ Architecture

```
devorch/
├── cmd/
│   ├── devorch/       # CLI entry point
│   └── devorchd/      # Daemon entry point
├── internal/
│   ├── provider/      # LLM providers (openai, anthropic, ollama, etc.)
│   ├── router/        # Intelligent routing with OkAON
│   ├── okaon/         # OkAON learning & Thompson sampling
│   ├── bench/         # Benchmarking framework
│   ├── modelresolver/ # Model capability resolution
│   ├── delegate/      # Multi-provider delegation
│   ├── background/    # Background task management
│   ├── platform/      # Platform detection (OS, arch, HW)
│   ├── storage/       # SQLite storage layer
│   └── wire/          # Dependency injection
└── bin/               # Build outputs
```

### Key Components

- **OkAON (Outcome-KPI-Aware Orchestration Network)**: Local learning system that records model performance and uses bandit algorithms for intelligent routing
- **Router**: Multi-strategy routing with fallback chains, drift detection, and quality gates
- **Bench**: Comprehensive benchmarking with latency, cost, and quality metrics
- **ModelResolver**: Maps model capabilities across providers
- **Delegate**: Handles multi-provider failover and load balancing

## 📊 Database Schema

22 migrations create the following key tables:

| Table | Purpose |
|-------|---------|
| `okaon_runs` | Model execution records |
| `okaon_work` | Work item tracking |
| `okaon_quality` | Quality evaluations |
| `okaon_reward` | Learning rewards |
| `okaon_arm_stats` | Bandit arm statistics |
| `router_policy` | Routing policies |
| `bench_runs` | Benchmark results |
| `model_stats` | Model performance stats |
| `experiments` | A/B test experiments |

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/okaon/...
go test ./internal/router/...
```

## 📝 Development

### Project Statistics

- **248 Go source files**
- **22 SQL migrations**
- **6 platform builds** (darwin/linux/windows × amd64/arm64)
- **~14MB binary size** (single static binary)

### Code Structure

```bash
# Count Go files
find . -name "*.go" | wc -l  # 248

# List migrations
ls internal/storage/sqlite/migrations/  # 0001-0022

# Check binary size
ls -lh bin/devorch-*  # ~14MB each
```

## 📄 License

MIT License - see LICENSE file for details.

## 🔗 References

- [OkAON Design Document](docs/phase20.md)
- [Router Policy Design](docs/phase15.md)
- [Benchmark Framework](docs/phase6.md)
- [Model Resolver](docs/phase26.md)

