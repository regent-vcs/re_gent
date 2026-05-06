CONCEPT & VISION
================

Why does AI agent activity need its own version control?

  Because agents are autonomous contributors, just like human developers. We 
  gave humans git. We gave agents write access to our code. But we never 
  gave ourselves git for them. When an agent breaks something, you have no 
  audit trail — just "it was working five minutes ago."


What problem does re_gent solve?

  The agent black box. Your agent makes 47 changes across 12 files. Three 
  hours later, tests fail. Which change broke it? Which prompt caused it?
  
  re_gent: "rgt blame" shows the exact step. "rgt show" shows full context. 
  "rgt rewind" undoes it.


Why not just use git commits?

  Agents work at different granularity. A human makes deliberate commits. An 
  agent makes 50 edits from one prompt. Auto-committing every tool call 
  creates noise. Committing nothing loses the audit trail. re_gent captures 
  the agent's natural unit of work without polluting git.


Should we treat agents like developers on the team?

  If they write code, yes. Their work should be auditable (blame), 
  reversible (rewind), and attributed (which prompt, which session). You 
  wouldn't let a human commit without record. Don't let agents either.


What's the mental model: re_gent vs git?

  Git = version control for human intent (commits, branches, PRs)
  re_gent = version control for agent actions (steps, sessions, prompts)
  
  Git answers: "What did we ship?"
  re_gent answers: "What did the agent do? Why?"


What's the long-term vision?

  AI agents will write most code within 5 years. You'll have Agent A 
  refactoring, Agent B writing tests, Agent C migrating databases. You 
  review and merge their work. re_gent is infrastructure for that world — 
  the git of agentic development.


THE PROBLEM SPACE
=================

What's the "agent undo problem"?

  You're pairing with Claude. It refactors your auth system. Three prompts 
  later, you realize it broke session handling. Your options: manual 
  archaeology, git revert (loses good work), or start over.
  
  re_gent: "rgt blame auth.go" → find breaking step → "rgt rewind" → back to 
  good state, conversation intact.


What's the "which prompt did this" problem?

  Your agent made 47 changes. One introduced a bug. Which prompt? Git blame 
  shows "2 hours ago" but not WHY.
  
  re_gent blame shows: "Step a7f3e891 - prompt: 'refactor error handling' - 
  tool: Edit" — now you know exactly what to fix.


What's the "agent session chaos" problem?

  Two Claude windows on the same project. One fixing bugs, one adding 
  features. Both editing files. Which session did what? Did they conflict?
  
  re_gent tracks sessions as branches. "rgt sessions" shows what each did, 
  which files they touched, and where they overlap.


Why can't I just use /compact or start fresh?

  You can, but you lose history. /compact deletes messages. Starting fresh 
  loses conversation. Both destroy the audit trail.
  
  re_gent captures everything in content-addressed storage. Even if Claude 
  compacts, re_gent has the full record.


PRACTICAL USE CASES
===================

When would I use "rgt blame"?

  Your API returns 500 errors. You didn't write this — the agent did. Run 
  "rgt blame handler.go:78" → see "Step 1f9c4e23 - 'refactor handler to be 
  modular' - extracted processRequest()" → now you know the refactor broke it.


When would I use "rgt rewind"?

  Claude refactored auth (good), added rate limiting (good), "optimized" 
  queries (broke production). Changes are interleaved. Run "rgt log" → find 
  good state → "rgt rewind 5e2b8c71" → back to working code.


When would I use "rgt sessions"?

  You ran three agent sessions yesterday. One succeeded, one broke tests, one 
  is still running. "rgt sessions --verbose" shows: what each did, how many 
  files, duration, and which files overlap.


When would I use "rgt log --graph"?

  Two Claude windows running. One refactoring API, one adding caching. 
  "rgt log --graph --all" shows visual tree of how their work diverged and 
  where they might conflict.


GETTING STARTED
===============

How do I install it?

  brew install regent-vcs/tap/rgt (or: cargo install rgt)
  Then: rgt init in your project.


Which AI tools work?

  Currently: Claude Code (all platforms)
  Planned: Cursor, Cline, Continue, Aider


Do I need to change my workflow?

  No. re_gent runs silently via hooks.


TECHNICAL
=========

Does it send data anywhere?

  No. Everything stays local. No telemetry, no cloud.


Does it slow down my agent?

  No. Adds 10-50ms per tool call.


How much disk space?

  10-50MB for several hours of work.


Does it replace git?

  No. Complementary. Git for human workflows, re_gent for agent audit trails.


Should I commit .regent/ to git?

  Usually no. Exception: if your team wants shared agent audit trails.


Can I run multiple agent sessions safely?

  Yes. Each gets its own branch. Conflicts are detected.


ADVANCED
========

Can I export history?

  rgt export --format json
  rgt export --redact-secrets (for sharing)


Can I use it in CI/CD?

  Yes. Run agents in CI, then "rgt log --format markdown" for PR descriptions.


What's coming next?

  v1 (Q3 2026): Conversation rewind, session merge, monorepo performance
  v2 (Q4 2026): Multi-tool orchestration, remote storage
  v3 (2027): Replay mode, model comparison, team features


Will it always be free?

  CLI: always free and open source (MIT)
  Potential paid: hosted sync, team SaaS, enterprise support


COMMUNITY
=========

How do I contribute?

  github.com/regent-vcs/regent


Where can I get help?

  GitHub Discussions or Discord (coming soon)


How do I report bugs?

  github.com/regent-vcs/regent/issues
