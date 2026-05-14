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


Why not just use git commits?

  Agents work at different granularity. A human makes deliberate commits. An 
  agent makes 50 edits from one prompt. Auto-committing every tool call 
  creates noise. Committing nothing loses the audit trail. re_gent captures 
  the agent's natural unit of work without polluting git.


Should we treat agents like developers on the team?

  If they write code, yes. Their work should be auditable (blame), 
  inspectable (show), and attributed (which prompt, which session). You 
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


When would I use "rgt show"?

  You found a suspicious change via blame. Run "rgt show <step-hash>" to see 
  the full conversation context: what the user asked, what the agent decided 
  to do, and the exact tool calls it made.


When would I use "rgt sessions"?

  You ran three agent sessions yesterday. One succeeded, one broke tests, one 
  is still running. "rgt sessions" shows: what each did, how many 
  files, duration, and which files overlap.


When would I use "rgt log --graph"?

  Two Claude windows running. One refactoring API, one adding caching. 
  "rgt log --graph --all" shows visual tree of how their work diverged and 
  where they might conflict.


GETTING STARTED
===============

How do I install it?

  brew install regent-vcs/tap/rgt
  Then: rgt init in your project.


Which AI tools work?

  Currently: Claude Code, Codex CLI, and OpenCode.
  Planned: Cursor, Cline, Continue.


Do I need to change my workflow?

  No. re_gent runs silently via hooks. Zero configuration after `rgt init`.


TECHNICAL
=========

Does it send data anywhere?

  No. Everything stays local. No telemetry, no cloud.


Does it slow down my agent?

  No. Adds 10-50ms per tool call.


How much disk space?

  10-50MB for several hours of work. Content-addressed storage deduplicates 
  identical file content automatically.


Does it replace git?

  No. Complementary. Git for human workflows, re_gent for agent audit trails.


Should I commit .regent/ to git?

  Usually no. Add it to .gitignore. Exception: if your team wants shared 
  agent audit trails.


Can I run multiple agent sessions safely?

  Yes. Each gets its own branch. CAS-based ref updates prevent corruption 
  under concurrent writes.


ADVANCED
========

Can I use it in CI/CD?

  Yes. Run agents in CI, then "rgt log --json" for structured output.


What's coming next?

  Planned: Rewind/fork operations, additional tool adapters (Cursor, Cline, 
  Continue), session sharing and merge, garbage collection.
  See ROADMAP.md for details.


Will it always be free?

  CLI: always free and open source (Apache 2.0).
  Potential paid: hosted sync, team SaaS, enterprise support.


COMMUNITY
=========

How do I contribute?

  github.com/regent-vcs/regent — see CONTRIBUTING.md


Where can I get help?

  GitHub Discussions or Discord: https://discord.gg/Unf24KMh


How do I report bugs?

  github.com/regent-vcs/regent/issues
