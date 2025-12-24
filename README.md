## Video Walkthrough
**Video Link**: https://www.loom.com/looms/videos/subspace-assginment-06105ef110ec49ffaedf3102dcbdf1fc

> **Note**: The video demonstrates the tool in a controlled environment for educational purposes only. Do not use this tool against real LinkedIn accounts.

## Overview

This is a sophisticated LinkedIn automation proof-of-concept built with Go, demonstrating advanced browser automation, anti-detection techniques, and clean software architecture. The project showcases:

- **Browser Automation**: Using the Rod library for Chrome DevTools Protocol automation
- **Stealth Techniques**: 8+ anti-detection mechanisms to mimic human behavior
- **Modular Architecture**: Clean separation of concerns with well-defined packages
- **State Management**: SQLite database for persistence and resumption
- **Configuration Management**: YAML + environment variables
- **Structured Logging**: Comprehensive logging with zap

## Project Structure
### Key Files

- **cmd/main.go**: Application entry point with main loop
- **internal/auth/auth.go**: Login, session management, security challenge detection
- **internal/search/search.go**: Profile search, URL extraction, pagination
- **internal/connection/connection.go**: Connection request automation
- **internal/messaging/messaging.go**: Message sending and template rendering
- **internal/stealth/stealth.go**: All anti-detection techniques
- **internal/storage/storage.go**: SQLite database operations
- **internal/config/config.go**: Configuration loading and validation
- **internal/logger/logger.go**: Structured logging with zap

## 📁 Project Structure

```
linkedin-automation/
├── cmd/
│   └── main.go                          ✅ Main application entry point
├── internal/
│   ├── auth/
│   │   └── auth.go                      ✅ Authentication & session management
│   ├── search/
│   │   └── search.go                    ✅ Profile search & collection
│   ├── connection/
│   │   └── connection.go                ✅ Connection request automation
│   ├── messaging/
│   │   └── messaging.go                 ✅ Message automation
│   ├── stealth/
│   │   └── stealth.go                   ✅ 8+ anti-detection techniques
│   ├── config/
│   │   └── config.go                    ✅ Configuration management
│   ├── storage/
│   │   └── storage.go                   ✅ SQLite persistence
│   └── logger/
│       └── logger.go                    ✅ Structured logging
├── config/
│   └── config.yaml                      ✅ Configuration file
├── .env.example                          ✅ Environment variables template
├── .gitignore                            ✅ Git ignore rules
├── go.mod                                ✅ Go modules
├── LICENSE                               ✅ MIT License
├── Makefile                              ✅ Build automation
├── README.md                             ✅ Comprehensive documentation
```

## 🚀 Setup and Installation

### Prerequisites

Before running this project, ensure you have the following installed:

- **Go 1.24+**: [Download and install Go](https://go.dev/dl/)
- **Git**: [Download Git](https://git-scm.com/downloads)
- **Chrome/Chromium**: Required for browser automation
- **GCC/MinGW**: Required for SQLite (CGO)
  - Windows: Install [MinGW-w64](https://www.mingw-w64.org/) or [TDM-GCC](https://jmeubank.github.io/tdm-gcc/)
  - Linux: `sudo apt-get install build-essential`
  - macOS: Xcode Command Line Tools (`xcode-select --install`)

### Installation Steps

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd linkedin-automation
   ```

2. **Install Go dependencies**
   ```bash
   go mod download
   ```
   
   Or using Make:
   ```bash
   make deps
   ```

3. **Configure environment variables**
   
   Copy the example environment file:
   ```bash
   cp .env.example .env
   ```
   
   Edit `.env` and add your credentials:
   ```env
   LINKEDIN_EMAIL=your.email@example.com
   LINKEDIN_PASSWORD=your_password_here
   LOG_LEVEL=info
   DATABASE_PATH=./linkedin.db
   ```
   
   > ⚠️ **Security Warning**: Never commit your `.env` file with real credentials!

4. **Configure search parameters** (Optional)
   
   Edit `config/config.yaml` to customize:
   - Job titles to search for
   - Companies to target
   - Locations
   - Connection request templates
   - Rate limiting settings

5. **Create required directories**
   ```bash
   mkdir -p logs sessions
   ```

## 🎯 Running the Application

### Using Make (Recommended)

```bash
# Build and run
make run

# Or build separately
make build
./linkedin-automation.exe    # Windows
./linkedin-automation        # Linux/macOS
```

### Using Go directly

```bash
# Run without building
go run cmd/main.go

# Build and run
go build -o linkedin-automation.exe cmd/main.go
./linkedin-automation.exe
```

### Command Line Options

The application uses configuration from:
1. Environment variables (`.env` file)
2. YAML configuration (`config/config.yaml`)
3. Default values

### Expected Behavior

When you run the application:

1. **Browser Launch**: Chrome will open (visible by default for debugging)
2. **Login**: Automated login to LinkedIn
3. **Search**: Searches for profiles based on your config
4. **Connection Requests**: Sends personalized connection requests
5. **Rate Limiting**: Respects delays and limits to avoid detection
6. **Logging**: All actions logged to console and `logs/` directory

### Example Output

```
2024-12-24T10:30:15.123Z INFO  Starting LinkedIn automation
2024-12-24T10:30:16.456Z INFO  Browser launched successfully
2024-12-24T10:30:20.789Z INFO  Login successful
2024-12-24T10:30:25.123Z INFO  Searching for: Software Engineer at Google
2024-12-24T10:30:30.456Z INFO  Found 47 profiles
2024-12-24T10:30:35.789Z INFO  Sent connection request to John Doe
2024-12-24T10:32:40.123Z INFO  Waiting 125 seconds before next action...
```

## 🛠️ Troubleshooting

### Common Issues

**1. "gcc: command not found" or CGO errors**
   - Install GCC/MinGW for your platform (required for SQLite)
   - Ensure GCC is in your PATH

**2. Browser fails to launch**
   - Check Chrome/Chromium installation
   - Try setting `HEADLESS_MODE=false` in `.env`

**3. Login fails**
   - Verify credentials in `.env`
   - LinkedIn may require 2FA - handle manually in browser
   - Check if LinkedIn detects automation (use stealth settings)

**4. "Rate limit exceeded"**
   - Reduce `DAILY_CONNECTION_LIMIT` and `HOURLY_ACTION_LIMIT`
   - Increase `MIN_ACTION_DELAY_SEC` and `MAX_ACTION_DELAY_SEC`

**5. Database errors**
   - Delete `linkedin.db` to start fresh
   - Check write permissions in the project directory

### Debugging

Enable debug logging in `.env`:
```env
LOG_LEVEL=debug
LOG_TO_FILE=true
```

Check logs in the `logs/` directory for detailed information.

## ⚙️ Configuration Reference

### Environment Variables (`.env`)

| Variable | Description | Default |
|----------|-------------|---------|
| `LINKEDIN_EMAIL` | Your LinkedIn email | (required) |
| `LINKEDIN_PASSWORD` | Your LinkedIn password | (required) |
| `LOG_LEVEL` | Logging level: debug/info/warn/error | info |
| `DATABASE_PATH` | SQLite database location | ./linkedin.db |
| `DAILY_CONNECTION_LIMIT` | Max connections per day | 50 |
| `HEADLESS_MODE` | Run browser in background | false |

### YAML Configuration (`config/config.yaml`)

- **Search criteria**: Job titles, companies, locations
- **Connection templates**: Personalized message templates
- **Rate limiting**: Delays and limits
- **Stealth settings**: Anti-detection parameters

## 🧹 Cleanup

To remove all generated files:

```bash
make clean
```

This removes:
- Compiled binary
- Database file
- Log files
- Session cookies