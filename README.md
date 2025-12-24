## Video Walkthrough
**Video Link**: [INSERT VIDEO LINK HERE]

**Complete Setup and Feature Demonstration**

Watch the comprehensive walkthrough video that covers:
- ✅ **Tool Setup**: Installing prerequisites, dependencies, and environment configuration
- ✅ **Configuration**: Setting up config.yaml, .env file, and customizing parameters
- ✅ **Execution**: Running the automation tool and observing its behavior
- ✅ **Key Features**: Demonstration of all core functionality and anti-detection techniques
- ✅ **Database & Logging**: Viewing stored data and examining logs
- ✅ **Troubleshooting**: Common issues and their solutions


> **Note**: The video demonstrates the tool in a controlled environment for educational purposes only. Do not use this tool against real LinkedIn accounts.

---

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