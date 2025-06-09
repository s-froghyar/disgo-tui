# Discogs TUI

A high-performance terminal user interface for managing your Discogs collection, wishlist, and orders with secure OAuth authentication and encrypted token storage.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Navigation](#navigation)
- [Architecture](#architecture)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Overview

Discogs TUI provides a fast, keyboard-driven interface for browsing and managing your Discogs music collection directly from the terminal. Built with Go and leveraging the official Discogs API, it offers secure authentication, automatic token management, and a responsive grid-based layout for optimal viewing experience.

### Key Benefits

- **Zero Configuration**: Automatic OAuth flow with encrypted token storage
- **Offline-First**: Cached data with intelligent refresh mechanisms  
- **Keyboard-Driven**: Efficient navigation without mouse dependency
- **Secure**: AES-encrypted credential storage with automatic token refresh
- **Fast**: Concurrent image loading with graceful fallbacks

## Features

### Core Functionality
- ✅ **Collection Management**: Browse your complete Discogs collection
- ✅ **Wishlist Tracking**: View and manage your want list
- ✅ **Order History**: Track your purchase history and order status
- ✅ **Release Details**: View comprehensive release information with cover art
- ✅ **Grid Navigation**: Configurable grid layout for optimal viewing

### Authentication & Security
- ✅ **OAuth 2.0**: Secure Discogs API authentication
- ✅ **Encrypted Storage**: AES-encrypted token persistence
- ✅ **Auto-Refresh**: Automatic token renewal and error handling
- ✅ **Environment Isolation**: Secure credential management

### User Experience
- ✅ **Responsive Design**: Adaptive layout for different terminal sizes
- ✅ **Image Loading**: Concurrent thumbnail fetching with fallbacks
- ✅ **Real-time Updates**: Configurable auto-refresh intervals
- ✅ **Error Recovery**: Graceful handling of network and API issues

## Prerequisites

### System Requirements
- **Go**: Version 1.22.0 or higher
- **Terminal**: Modern terminal with image support (recommended)
- **Network**: Internet connection for API access
- **OS**: Linux, macOS, or Windows

### Discogs API Credentials
1. Create a Discogs account at [discogs.com](https://www.discogs.com)
2. Navigate to [Developer Settings](https://www.discogs.com/settings/developers)
3. Create a new application to obtain:
   - Consumer Key
   - Consumer Secret

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/your-username/disgo-tui.git
cd disgo-tui

# Build the application
go build -o disgo-tui ./cmd/main.go

# Make executable
chmod +x disgo-tui
```

### Using Go Install

```bash
go install github.com/your-username/disgo-tui/cmd/main@latest
```

## Configuration

### Environment Variables Setup

Create the environment configuration file:

```bash
# Create the environment script
cp tui_envs.sh.example tui_envs.sh
```

Edit `tui_envs.sh` with your credentials:

```bash
#!/bin/bash

# Environment Variables Export Script
# Usage: source ./tui_envs.sh or . ./tui_envs.sh

# Discogs API credentials
export DISCOGS_API_CONSUMER_KEY="your_consumer_key_here"
export DISCOGS_API_CONSUMER_SECRET="your_consumer_secret_here"

# OAuth callback port (used for max 5 minutes during authentication)
export LOCAL_PORT=8081

echo "Environment variables loaded successfully"
```

### Application Configuration

The application uses `configs/conf.yaml` for UI settings:

```yaml
grid:
  rows: 2      # Number of rows in the grid layout
  cols: 2      # Number of columns in the grid layout
update_frequency: 10  # Auto-refresh interval in seconds
```

### Security Considerations

- **Credentials**: Never commit `tui_envs.sh` with real credentials
- **Port Selection**: Choose an available port (default: 8081)
- **Token Storage**: Tokens are encrypted and stored in `~/.config/discogs-tui/`

## Usage

### First Run

1. **Load Environment Variables**:
   ```bash
   source ./tui_envs.sh
   ```

2. **Start the Application**:
   ```bash
   ./disgo-tui
   ```

3. **Complete OAuth Flow**:
   - Application opens OAuth URL automatically
   - Authorize the application in your browser
   - Return to terminal once authentication completes
   - Tokens are automatically saved for future use

### Subsequent Runs

```bash
# Load environment (if not persistent)
source ./tui_envs.sh

# Start application (tokens loaded automatically)
./disgo-tui
```

### Authentication Flow

```
┌─ Initial Setup ─────────────────────────────────┐
│ 1. Check for existing tokens                    │
│ 2. If none found, start OAuth flow             │
│ 3. Open browser for authorization              │
│ 4. Save encrypted tokens locally               │
│ 5. Verify authentication with Discogs API      │
└─────────────────────────────────────────────────┘
```

## Navigation

### Keyboard Controls

| Key Combination | Action |
|-----------------|--------|
| `Ctrl+A` | Focus on menu navigation |
| `Ctrl+D` | Focus on preview grid |
| `Arrow Keys` | Navigate grid items |
| `Enter` | Open release details modal |
| `0` | Switch to Collection view |
| `1` | Switch to Wishlist view |
| `2` | Switch to Orders view |
| `q` | Quit application |
| `Ctrl+C` | Force quit |

### Interface Layout

```
┌─ Menu ──────┐ ┌─ Preview Grid ─────────────────┐
│ Collection  │ │ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ │
│ Wishlist    │ │ │ ♫   │ │ ♫   │ │ ♫   │ │ ♫   │ │
│ Orders      │ │ │Art 1│ │Art 2│ │Art 3│ │Art 4│ │
│ Quit        │ │ └─────┘ └─────┘ └─────┘ └─────┘ │
│             │ │ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ │
│ ┌─────────┐ │ │ │ ♫   │ │ ♫   │ │ ♫   │ │ ♫   │ │
│ │  LOGO   │ │ │ │Art 5│ │Art 6│ │Art 7│ │Art 8│ │
│ └─────────┘ │ │ └─────┘ └─────┘ └─────┘ └─────┘ │
└─────────────┘ └───────────────────────────────────┘
┌─ Status Bar ──────────────────────────────────────┐
│ Navigate: Arrow keys • Enter: Details • Ctrl+C: Exit │
└───────────────────────────────────────────────────────┘
```

## Architecture

### Project Structure

```
disgo-tui/
├── cmd/
│   └── main.go                 # Application entry point
├── configs/
│   ├── conf.yaml              # UI configuration
│   └── config.go              # Configuration loader
├── internal/
│   ├── client/
│   │   ├── discogs.go         # Discogs API client
│   │   └── http.go            # HTTP client with OAuth
│   ├── dto/
│   │   └── discogs.go         # Data transfer objects
│   └── tui/
│       ├── events.go          # Event handlers
│       ├── keyboard.go        # Key mappings
│       ├── logo.go            # Logo rendering
│       └── tui.go             # Main TUI logic
├── tui_envs.sh                # Environment variables
├── go.mod                     # Go module definition
└── README.md                  # This file
```

### Core Components

#### Authentication Layer
- **OAuth 1.0 Flow**: Secure three-legged authentication
- **Token Management**: Encrypted storage with automatic refresh
- **Error Handling**: Graceful fallbacks and re-authentication

#### API Client Layer
- **Rate Limiting**: Respectful API usage patterns
- **Context Support**: Timeout and cancellation handling
- **Error Recovery**: Automatic retry with exponential backoff

#### TUI Layer
- **Grid System**: Responsive layout management
- **Event Handling**: Keyboard and focus management
- **State Management**: Efficient data synchronization

### Data Flow

```
┌─ User Input ─┐    ┌─ TUI Layer ─┐    ┌─ API Client ─┐    ┌─ Discogs API ─┐
│   Keyboard   │───▶│   Events    │───▶│   HTTP Req   │───▶│   Response    │
│   Actions    │    │   Handlers  │    │   w/ OAuth   │    │   JSON Data   │
└──────────────┘    └─────────────┘    └──────────────┘    └───────────────┘
                            │                  │
                            ▼                  ▼
                    ┌─ State Mgmt ─┐    ┌─ Token Store ─┐
                    │   Grid Data  │    │   Encrypted   │
                    │   UI State   │    │   Persistent  │
                    └──────────────┘    └───────────────┘
```

## Development

### Building from Source

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build for current platform
go build -o disgo-tui ./cmd/main.go

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o disgo-tui-linux ./cmd/main.go
GOOS=darwin GOARCH=amd64 go build -o disgo-tui-macos ./cmd/main.go
GOOS=windows GOARCH=amd64 go build -o disgo-tui.exe ./cmd/main.go
```

### Development Environment

```bash
# Enable development mode
export DISCOGS_TUI_DEBUG=true

# Use development port
export LOCAL_PORT=3000

# Run with hot reload (using air)
air
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint code
golangci-lint run

# Security scan
gosec ./...

# Dependency check
go mod verify
```

## Troubleshooting

### Common Issues

#### Authentication Problems

**Issue**: OAuth flow fails or hangs
```bash
# Solution 1: Check port availability
netstat -an | grep :8081

# Solution 2: Clear stored tokens
rm -rf ~/.config/discogs-tui/

# Solution 3: Verify credentials
echo $DISCOGS_API_CONSUMER_KEY
```

**Issue**: "Invalid token" errors
```bash
# Clear tokens and re-authenticate
rm -rf ~/.config/discogs-tui/
unset DISCOGS_TOKEN DISCOGS_TOKEN_SECRET
./disgo-tui
```

#### Network Issues

**Issue**: API timeouts or connection errors
```bash
# Check internet connectivity
curl -I https://api.discogs.com/

# Verify API status
curl https://api.discogs.com/
```

#### Performance Issues

**Issue**: Slow image loading
```yaml
# Reduce grid size in configs/conf.yaml
grid:
  rows: 1
  cols: 2
```

**Issue**: High memory usage
```bash
# Monitor memory usage
ps aux | grep disgo-tui

# Reduce update frequency
update_frequency: 30  # seconds
```

### Debug Mode

Enable verbose logging:

```bash
export DISCOGS_TUI_DEBUG=true
export DISCOGS_TUI_LOG_LEVEL=debug
./disgo-tui 2>&1 | tee debug.log
```

### Log Files

Default log locations:
- **Linux**: `~/.local/share/discogs-tui/logs/`
- **macOS**: `~/Library/Logs/discogs-tui/`
- **Windows**: `%APPDATA%\discogs-tui\logs\`

## Contributing

### Getting Started

1. **Fork the Repository**
2. **Create Feature Branch**:
   ```bash
   git checkout -b feature/amazing-feature
   ```
3. **Make Changes**: Follow Go best practices
4. **Add Tests**: Maintain test coverage
5. **Submit Pull Request**: Include detailed description

### Code Standards

- **Go Version**: Minimum 1.22.0
- **Formatting**: Use `gofmt` and `goimports`
- **Linting**: Pass `golangci-lint` checks
- **Testing**: Maintain >80% test coverage
- **Documentation**: Include comprehensive comments

### Pull Request Checklist

- [ ] Tests pass: `go test ./...`
- [ ] Lints clean: `golangci-lint run`
- [ ] Documentation updated
- [ ] Changelog entry added
- [ ] Breaking changes noted

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Made with ♫ for the vinyl community**

For questions, issues, or feature requests, please [open an issue](https://github.com/your-username/disgo-tui/issues).