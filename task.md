# task.md

# Dream OpenCode Environment Setup Checklist

## Objective

Build a clean, production-ready OpenCode environment optimized for:

* Laravel
* Vue 3
* TypeScript
* PostgreSQL
* Docker
* Ubuntu Linux

---

# Phase 1 — Foundation (Highest Priority)

These MCPs should always be available.

## Documentation

* [x] Context7
* [ ] Verify Context7 is working
* [ ] Test with latest Laravel documentation
* [ ] Test with latest Vue documentation

---

## Code Intelligence

* [x] CodeGraph
* [ ] Index current project
* [ ] Verify dependency graph
* [ ] Test architecture queries

---

## Language Server

* [x] LSP
* [ ] Verify Go to Definition
* [ ] Verify Rename Symbol
* [ ] Verify Find References
* [ ] Verify Diagnostics

---

## Filesystem

* [ ] Install Filesystem MCP
* [ ] Restrict allowed directories
* [ ] Test file creation
* [ ] Test file editing
* [ ] Test file rename
* [ ] Test file deletion

---

## GitHub

* [ ] Install GitHub MCP
* [ ] Authenticate
* [ ] Test repository access
* [ ] Test PR reading
* [ ] Test Issues
* [ ] Test GitHub Actions

---

# Phase 2 — Development Layer

## PostgreSQL

* [ ] Install PostgreSQL MCP
* [ ] Connect local database
* [ ] Read schema
* [ ] Inspect indexes
* [ ] Explain queries
* [ ] Verify migrations

---

## Docker

* [ ] Install Docker MCP
* [ ] Connect Docker daemon
* [ ] List containers
* [ ] Read logs
* [ ] Restart containers
* [ ] Test Docker Compose

---

## Playwright

* [ ] Install Playwright MCP
* [ ] Install browsers
* [ ] Capture screenshot
* [ ] Fill forms
* [ ] Test login
* [ ] Verify dashboard
* [ ] Run E2E tests

---

# Phase 3 — Knowledge Layer

## Grep

* [x] Installed
* [ ] Search Laravel implementation
* [ ] Search Vue implementation
* [ ] Search authentication examples

---

## Firecrawl

* [ ] Install Firecrawl MCP
* [ ] Configure API key
* [ ] Crawl documentation
* [ ] Extract release notes
* [ ] Test API documentation scraping

---

## WebSearch

* [x] Installed
* [ ] Verify general search
* [ ] Compare with Firecrawl
* [ ] Use only when Context7 cannot answer

---

# Phase 4 — Productivity

## OpenCode Mem

* [ ] Install
* [ ] Save coding conventions
* [ ] Save project architecture
* [ ] Verify memory retrieval

---

## Dynamic Context Pruning

* [ ] Install
* [ ] Verify long-session cleanup
* [ ] Measure token reduction

---

## Envsitter Guard

* [ ] Install
* [ ] Protect .env
* [ ] Protect API keys
* [ ] Verify secret blocking

---

# Phase 5 — Optional MCPs

Install only when required.

## Production

* [ ] Sentry

---

## UI / Design

* [ ] Figma

---

## Team Collaboration

* [ ] Linear

---

## SaaS Integrations

* [ ] Composio

---

## Isolated Development

* [ ] Daytona Sandbox

---

# OpenCode Configuration

## Global MCPs

Always Enabled

* [ ] Context7
* [ ] CodeGraph
* [ ] LSP
* [ ] Filesystem
* [ ] GitHub

---

## Project MCPs

Enable per project

* [ ] PostgreSQL
* [ ] Docker
* [ ] Playwright
* [ ] Firecrawl
* [ ] Grep
* [ ] WebSearch

---

## Optional MCPs

Disabled by default

* [ ] OpenCode Mem
* [ ] Dynamic Context Pruning
* [ ] Envsitter Guard
* [ ] Sentry
* [ ] Figma
* [ ] Linear
* [ ] Composio
* [ ] Daytona Sandbox

---

# Validation Tests

## Documentation

* [ ] Ask latest Laravel feature
* [ ] Ask latest Vue API
* [ ] Verify Context7 response

---

## Repository

* [ ] Read Pull Request
* [ ] Create Issue
* [ ] Search commits

---

## Filesystem

* [ ] Create file
* [ ] Modify file
* [ ] Rename file
* [ ] Delete file

---

## Database

* [ ] Explain schema
* [ ] Generate migration
* [ ] Inspect relationships

---

## Docker

* [ ] List containers
* [ ] Restart container
* [ ] Read logs

---

## Browser

* [ ] Open application
* [ ] Login
* [ ] Verify UI
* [ ] Take screenshot

---

# Workflow Validation

## Feature Development

* [ ] Requirements → Context7
* [ ] Architecture → CodeGraph
* [ ] Navigation → LSP
* [ ] Implementation → Filesystem
* [ ] Database → PostgreSQL
* [ ] Runtime → Docker
* [ ] UI Testing → Playwright
* [ ] Commit → GitHub

---

## Bug Fix

* [ ] Read GitHub issue
* [ ] Analyze architecture
* [ ] Locate symbols
* [ ] Fix implementation
* [ ] Test automatically
* [ ] Commit changes

---

## Success Criteria

The environment should be able to:

* [ ] Understand project architecture
* [ ] Navigate code semantically
* [ ] Access latest documentation
* [ ] Search production examples
* [ ] Edit files safely
* [ ] Read GitHub repositories
* [ ] Query PostgreSQL
* [ ] Control Docker
* [ ] Automate browser testing
* [ ] Protect secrets
* [ ] Remember project conventions
* [ ] Minimize context usage
* [ ] Complete end-to-end development tasks with minimal manual intervention

---

# Target Completion

## Foundation

* [ ] Complete

## Development

* [ ] Complete

## Knowledge

* [ ] Complete

## Productivity

* [ ] Complete

## Optional

* [ ] Complete (as needed)

---

## Final Goal

Achieve a lean, high-performance OpenCode environment where the agent can:

1. **Understand** the codebase.
2. **Plan** implementations.
3. **Write** and refactor code.
4. **Manage** databases and containers.
5. **Test** changes automatically.
6. **Research** current documentation and real-world examples.
7. **Maintain** project context across long development sessions.

**Target Environment Score:** ⭐ 10/10
