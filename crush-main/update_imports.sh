#!/bin/bash

# Script to update all import paths after refactoring

cd /Users/apple/rolling-crush/crush-main

# Find all .go files and update imports
find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" -exec sed -i '' \
  -e 's|github.com/charmbracelet/crush/internal/httpserver|github.com/charmbracelet/crush/api/http|g' \
  -e 's|github.com/charmbracelet/crush/internal/server|github.com/charmbracelet/crush/api/ws|g' \
  -e 's|github.com/charmbracelet/crush/internal/auth|github.com/charmbracelet/crush/auth|g' \
  -e 's|github.com/charmbracelet/crush/internal/appconfig|github.com/charmbracelet/crush/pkg/config|g' \
  -e 's|github.com/charmbracelet/crush/internal/config|github.com/charmbracelet/crush/pkg/config|g' \
  -e 's|github.com/charmbracelet/crush/internal/db|github.com/charmbracelet/crush/store/postgres|g' \
  -e 's|github.com/charmbracelet/crush/internal/storage|github.com/charmbracelet/crush/store/storage|g' \
  -e 's|github.com/charmbracelet/crush/internal/sandbox|github.com/charmbracelet/crush/sandbox|g' \
  -e 's|github.com/charmbracelet/crush/internal/user|github.com/charmbracelet/crush/domain/user|g' \
  -e 's|github.com/charmbracelet/crush/internal/session|github.com/charmbracelet/crush/domain/session|g' \
  -e 's|github.com/charmbracelet/crush/internal/message|github.com/charmbracelet/crush/domain/message|g' \
  -e 's|github.com/charmbracelet/crush/internal/project|github.com/charmbracelet/crush/domain/project|g' \
  -e 's|github.com/charmbracelet/crush/internal/permission|github.com/charmbracelet/crush/domain/permission|g' \
  -e 's|github.com/charmbracelet/crush/internal/history|github.com/charmbracelet/crush/domain/history|g' \
  -e 's|github.com/charmbracelet/crush/internal/ansiext|github.com/charmbracelet/crush/internal/pkg/ansiext|g' \
  -e 's|github.com/charmbracelet/crush/internal/csync|github.com/charmbracelet/crush/internal/pkg/csync|g' \
  -e 's|github.com/charmbracelet/crush/internal/diff|github.com/charmbracelet/crush/internal/pkg/diff|g' \
  -e 's|github.com/charmbracelet/crush/internal/env|github.com/charmbracelet/crush/internal/pkg/env|g' \
  -e 's|github.com/charmbracelet/crush/internal/filepathext|github.com/charmbracelet/crush/internal/pkg/filepathext|g' \
  -e 's|github.com/charmbracelet/crush/internal/format|github.com/charmbracelet/crush/internal/pkg/format|g' \
  -e 's|github.com/charmbracelet/crush/internal/fsext|github.com/charmbracelet/crush/internal/pkg/fsext|g' \
  -e 's|github.com/charmbracelet/crush/internal/home|github.com/charmbracelet/crush/internal/pkg/home|g' \
  -e 's|github.com/charmbracelet/crush/internal/log|github.com/charmbracelet/crush/internal/pkg/log|g' \
  -e 's|github.com/charmbracelet/crush/internal/stringext|github.com/charmbracelet/crush/internal/pkg/stringext|g' \
  -e 's|github.com/charmbracelet/crush/internal/term|github.com/charmbracelet/crush/internal/pkg/term|g' \
  -e 's|github.com/charmbracelet/crush/internal/cmd|github.com/charmbracelet/crush/cmd|g' \
  {} \;

echo "Import paths updated successfully!"
