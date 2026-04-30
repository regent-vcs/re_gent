# Open Source Preparation - TODO Checklist

## P0 - Required for Launch (Do these first)

### Governance Files
- [ ] `LICENSE` - ✅ DONE (already created)
- [ ] `CODE_OF_CONDUCT.md` - Copy from https://www.contributor-covenant.org/version/2/1/code_of_conduct/
- [ ] `.golangci.yml` - Linter config (see plan for content)
- [ ] `CONTRIBUTING.md` - Development guide (~300-400 lines, see plan)
- [ ] `SECURITY.md` - Vulnerability reporting (see plan)
- [ ] `CHANGELOG.md` - Version history (see plan)

### GitHub Actions & CI
- [ ] `.github/workflows/ci.yml` - Test, lint, build on push/PR
- [ ] `.goreleaser.yaml` - Release configuration for GoReleaser

### Issue & PR Templates
- [ ] `.github/ISSUE_TEMPLATE/bug_report.yml`
- [ ] `.github/ISSUE_TEMPLATE/feature_request.yml`
- [ ] `.github/ISSUE_TEMPLATE/phase_implementation.yml`
- [ ] `.github/pull_request_template.md`

### README Enhancements
- [ ] Add badges (CI, Go version, License) at top
- [ ] Add demo placeholder section
- [ ] Add "Regent vs Git" comparison table
- [ ] Update Installation section (add "Coming Soon")
- [ ] Update Contributing section (link to CONTRIBUTING.md)

---

## P1 - Launch Week 1

### Additional Workflows
- [ ] `.github/workflows/release.yml` - Automated releases on tags

### Documentation
- [ ] `docs/architecture.md` - Technical deep-dive (~500-700 lines)
- [ ] `docs/commands.md` - CLI reference for all commands
- [ ] `docs/hook-integration.md` - Phase 2 preview (Claude Code integration)

---

## P2 - Month 1

### Extended Documentation
- [ ] `docs/faq.md` - Common questions and answers
- [ ] `docs/development.md` - Contributor deep-dive

### Assets Placeholders
- [ ] `assets/demos/README.md` - Instructions for creating demo GIFs
- [ ] `assets/diagrams/README.md` - Architecture diagram placeholders
- [ ] `assets/logo/README.md` - Logo placeholder

---

## Quick Reference

**All file content/templates are in the plan file:**
`/Users/shay/.claude/plans/i-want-to-prepare-clever-pony.md`

**Brand voice to maintain:**
- Technical, honest about POC status, opinionated with rationale
- Examples: "We gave agents write access to our codebases. We did not give ourselves git for it."

**Test before committing:**
```bash
# Run tests
go test ./...
go test -race ./...

# Test linter (after creating .golangci.yml)
golangci-lint run

# Test CI locally (after creating workflow)
# Push to a branch and verify GitHub Actions pass
```

**Verification:**
- [ ] All P0 files created
- [ ] CI workflow passes on GitHub
- [ ] Issue templates render correctly
- [ ] README badges link correctly
