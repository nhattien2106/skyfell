# Go Backend (Gin)

This is the backend for the Skyfell web crawler app.

## Features

- Built with Go and Gin
- REST API to accept a website URL
- Crawls the page and extracts key info (title, meta, etc.)
- Stores/retrieves data from MySQL

## Getting Started

1. Install dependencies:
   ```bash
   go mod tidy
   ```
2. Run the server:
   ```bash
   go run main.go
   ```

## Configuration

- Update MySQL connection settings in the code as needed.
