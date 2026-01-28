# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

---

## Project Context

**Project Name:** AnemoneSync
**Tech Stack:** Go 1.21+, Fyne (GUI), SQLite (sqlcipher), SMB2
**Primary Language:** Go
**Key Dependencies:** fyne.io/fyne/v2, github.com/hirochachacha/go-smb2, go-sqlcipher, fsnotify, zap
**Architecture Pattern:** Desktop app with system tray, background sync engine
**Development Environment:** Windows, MSYS2 MinGW64

---

## COMPILATION OBLIGATOIRE

**TOUJOURS utiliser MSYS2 MinGW64 GCC pour compiler ce projet !**

```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o anemonesync.exe ./cmd/anemonesync/
```

### Pourquoi ?
- Ce projet utilise Fyne (GUI) qui nécessite CGO
- TDM-GCC 10.3.0 (souvent dans le PATH par défaut) produit des binaires corrompus
- Erreur si mauvais compilateur : "CETTE APPLICATION NE PEUT PAS S'EXECUTER SUR VOTRE PC" ou "n'est pas une application Win32 valide"
- MSYS2 MinGW64 GCC 15.2.0 fonctionne correctement

### Ne JAMAIS faire :
```bash
go build -o anemonesync.exe ./cmd/anemonesync/  # MAUVAIS - utilise TDM-GCC
```

---

## Development Philosophy

### Golden Rule: Incremental Development

**NEVER write large amounts of code without validation.**

```
One module -> Test -> User validates -> Next module
```

**Per iteration limits:**
- 1-3 related files maximum
- ~50-150 lines of new code
- Must be independently testable

### Mandatory Stop Points

Claude MUST stop and wait for user validation after:
- Database connection/schema changes
- Authentication/authorization code
- Each API endpoint or route group
- File system or external service integrations
- Any security-sensitive code

**Stop format:**
```
[Module] complete.

**Test it:**
1. [Step 1]
2. [Step 2]
Expected: [Result]

Waiting for your validation before continuing.
```

### Code Hygiene Rules (MANDATORY)

**Goal: Application must be portable and deployable anywhere without code changes.**

**NEVER hardcode in source files:**
- Passwords, API keys, tokens, secrets
- Database credentials or connection strings
- Absolute paths (`C:\Users\...`, `/home/user/...`)
- IP addresses, hostnames, ports (production)
- Email addresses, usernames for services
- Environment-specific URLs (dev, staging, prod)

**ALWAYS use instead:**
- Environment variables (`.env` files, never committed)
- Configuration files (with `.example` templates)
- Relative paths or configurable base paths
- Secret managers for production (Vault, AWS Secrets, etc.)

### Development Order (Enforce)

1. **Foundation first** - Config, DB, Auth
2. **Test foundation** - Don't continue if broken
3. **Core features** - One by one, tested
4. **Advanced features** - Only after core works

### File Size Guidelines

**Target sizes (lines of code):**
- **< 300** : ideal
- **300-500** : acceptable
- **500-800** : consider splitting
- **> 800** : must split

**When to split a file:**
- Multiple unrelated concerns in the same file
- Hard to find functions/methods
- File has too many responsibilities
- Scrolling endlessly to find something

**Naming convention for split files:**
```
app.go           → Core struct, New(), Run(), Shutdown()
app_jobs.go      → Job-related methods
app_sync.go      → Sync-related methods
app_settings.go  → Config/settings methods
```

**Benefits of smaller files:**
- Easier to navigate and understand
- Cleaner git diffs
- Less merge conflicts
- Faster incremental compilation
- More focused tests

---

## Project Structure

```
cmd/anemonesync/       # Application entry point
internal/
├── app/               # Desktop app (Fyne + systray, watchers, scheduler)
├── sync/              # Sync engine (executor, retry, conflicts, worker pool)
├── smb/               # SMB client (connection, credentials, file ops)
├── database/          # SQLite persistence (encrypted)
├── scanner/           # File scanner (hash, metadata, exclusions)
├── cache/             # Change detection cache
└── cloudfiles/        # Windows Cloud Files API (Files On Demand)
```

---

## Session Management

### Quick Start

**Continue work:** `"continue"` or `"let's continue"`
**New session:** `"new session: Feature Name"`

### File Structure

- **SESSION_STATE.md** (root) - Overview and session index
- **sessions/session_XXX.md** - Detailed session logs

**Naming:** `session_001.md`, `session_002.md`, etc.

### SESSION_STATE.md Header (Required)

SESSION_STATE.md **must** start with this reminder block:

```markdown
# [Project] - Session State

> **Claude : Appliquer le protocole de session (CLAUDE.md)**
> - Creer/mettre a jour la session en temps reel
> - Valider apres chaque module avec : [Module] complete. **Test it:** [...] Waiting for validation.
> - Ne pas continuer sans validation utilisateur
```

### Session Template

```markdown
# Session XXX: [Feature Name]

## Meta
- **Date:** YYYY-MM-DD
- **Goal:** [Brief description]
- **Status:** In Progress / Blocked / Complete

## Current Module
**Working on:** [Module name]
**Progress:** [Status]

## Module Checklist
- [ ] Module planned (files, dependencies, test procedure)
- [ ] Code written
- [ ] Self-tested by Claude
- [ ] User validated <- **REQUIRED before next module**

## Completed Modules
| Module | Validated | Date |
|--------|-----------|------|
| DB Connection | Yes | YYYY-MM-DD |

## Next Modules (Prioritized)
1. [ ] [Next module]
2. [ ] [Following module]

## Technical Decisions
- **[Decision]:** [Reason]

## Issues & Solutions
- **[Issue]:** [Solution]

## Files Modified
- `path/file.ext` - [What/Why]

## Handoff Notes
[Critical context for next session]
```

### Session Rules

**MUST DO:**
1. Read CLAUDE.md and current session first
2. Update session file in real-time
3. Wait for validation after each module
4. Fix bugs before new features

**NEW SESSION when:**
- New major feature/module
- Current session goal complete
- Different project area

---

## Module Workflow

### 1. Plan (Before Coding)

```markdown
**Module:** [Name]
**Purpose:** [One sentence]
**Files:** [List]
**Depends on:** [Previous modules]
**Test procedure:** [How to verify]
**Security concerns:** [If any]
```

### 2. Implement

- Write minimal working code
- Include error handling
- Document as you go (headers, comments)

### 3. Validate

**Functional:**
- [ ] Runs without errors
- [ ] Expected output verified
- [ ] Errors handled gracefully

**Security (if applicable):**
- [ ] Input validated
- [ ] No hardcoded secrets, paths, or credentials
- [ ] Parameterized queries (SQL)
- [ ] Output encoded (XSS)

### 4. User Confirmation

**DO NOT proceed until user says "OK", "validated", or "continue"**

---

## Go Documentation Standards

### File Header
```go
// Package name provides [description].
package name
```

### Function Documentation
```go
// FunctionName does [what it does].
// It handles [key behaviors].
// Returns [what] or error if [conditions].
func FunctionName(param Type) (Result, error) {
```

### Struct Documentation
```go
// StructName represents [what it represents].
// It is used for [primary use case].
type StructName struct {
    Field Type // Field description
}
```

### Interface Documentation
```go
// InterfaceName defines [what behavior it defines].
type InterfaceName interface {
    // Method does [what].
    Method(param Type) error
}
```

---

## Pre-Launch Security Audit

### When to Run

**MANDATORY before any deployment or "project complete" status.**

### Security Audit Checklist

#### 1. Code Review (Full Scan)
- [ ] No hardcoded secrets (API keys, passwords, tokens)
- [ ] No hardcoded paths (use relative or configurable)
- [ ] No hardcoded credentials or connection strings
- [ ] No sensitive data in logs
- [ ] All user inputs validated and sanitized
- [ ] No debug/dev code left in production
- [ ] `.gitignore` excludes `.env` and sensitive files

#### 2. OWASP Top 10 Check
- [ ] **Injection** - SQL, NoSQL, OS command injection protected
- [ ] **Broken Auth** - Strong passwords, session management
- [ ] **Sensitive Data Exposure** - Encryption at rest and in transit
- [ ] **Broken Access Control** - Authorization verified on all routes
- [ ] **Security Misconfiguration** - Default credentials removed
- [ ] **XSS** - Output encoding, CSP headers
- [ ] **Vulnerable Components** - Dependencies updated, no known CVEs
- [ ] **Insufficient Logging** - Security events logged, logs protected

#### 3. Dependency Audit
```bash
# Go
go list -m all | nancy sleuth
govulncheck ./...
```
- [ ] All critical/high vulnerabilities addressed
- [ ] Outdated packages updated or justified

#### 4. Online Vulnerability Research
- [ ] Search CVE databases for stack components
- [ ] Check GitHub security advisories for dependencies
- [ ] Review recent security news for frameworks used

**Resources:**
- https://cve.mitre.org
- https://nvd.nist.gov
- https://github.com/advisories
- https://snyk.io/vuln

#### 5. Basic Penetration Testing
- [ ] SQL injection attempts on all inputs
- [ ] Auth bypass attempts (direct URL access, token manipulation)
- [ ] File path traversal tested
- [ ] Rate limiting verified (brute force protection)

#### 6. Configuration Security
- [ ] HTTPS enforced (if applicable)
- [ ] Security headers present (HSTS, CSP, X-Frame-Options, etc.)
- [ ] Cookies secured (HttpOnly, Secure, SameSite)
- [ ] Error pages don't leak stack traces
- [ ] Admin interfaces protected/hidden

### Audit Report Template

```markdown
# Security Audit Report

**Project:** [Name]
**Date:** YYYY-MM-DD
**Audited by:** [Claude / Human / Both]

## Summary
- Critical issues: X
- High issues: X
- Medium issues: X
- Low issues: X

## Findings

### [CRITICAL/HIGH/MEDIUM/LOW] Issue Title
- **Location:** [File:line or endpoint]
- **Description:** [What's wrong]
- **Risk:** [Impact if exploited]
- **Fix:** [How to resolve]
- **Status:** [ ] Fixed / [ ] Accepted risk

## Conclusion
[ ] Ready for launch
[ ] Requires fixes before launch
```

### Post-Audit Actions

1. **Critical/High issues** - Fix immediately, re-test
2. **Medium issues** - Fix before launch or document accepted risk
3. **Low issues** - Add to backlog
4. **Re-run audit** after fixes

---

## Git Workflow

### Branch Naming
```
feature/session-XXX-brief-name
fix/issue-description
```

### Commit Messages
```
type(scope): Brief summary

Details if needed.

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types:** feat, fix, docs, refactor, test, chore

### Examples
```
feat(sync): Add stop button for running sync
fix(smb): Handle connection timeout gracefully
docs: Update SESSION_STATE.md - Session 021 complete
```

---

## Testing

### Run Tests
```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go test ./...
```

### Run Specific Package
```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go test ./internal/sync/...
```

### Test Harness
```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o testharness.exe ./test/harness/
./testharness.exe              # All tests
./testharness.exe -job TEST1   # Specific job
./testharness.exe -list        # List scenarios
```

---

## Quick Commands

| Command | Action |
|---------|--------|
| `continue` | Resume current session |
| `new session: [name]` | Start new session |
| `save progress` | Update session file |
| `validate` | Mark current module as validated |
| `show plan` | Display remaining modules |
| `security audit` | Run full pre-launch security checklist |
| `dependency check` | Audit dependencies for vulnerabilities |
| `build` | Compile with correct GCC |

---

## Roadmap / TODO

- [x] Manifeste Anemone Server integration (accelerate remote scanning)
- [x] Display sync size in job list (update periodically)
- [x] First sync wizard / guided setup
- [x] CLI interface (`--list-jobs`, `--sync <job-id>`, `--sync-all`)
- [x] Cloud Files API hydration (Files On Demand - placeholders + hydration a la demande)

---

## File Standards

- **Encoding:** UTF-8 with LF (Unix) line endings
- **Timestamps:** ISO 8601 (YYYY-MM-DD HH:mm)
- **Time format:** 24-hour (HH:mm)

---

**Last Updated:** 2026-01-28
**Version:** 3.2.0
