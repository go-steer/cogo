---
title: Skills
weight: 4
---

Skills are reusable agent capabilities packaged as a directory with a `SKILL.md` file. The format is the same as Claude Code's, so you can drop in skills written for either tool.

## Layout

```
.agents/skills/
├── code-review/
│   ├── SKILL.md
│   └── (optional supporting files)
├── git-flow/
│   └── SKILL.md
└── deploy-checklist/
    └── SKILL.md
```

Each subdirectory of `.agents/skills/` becomes one invocable skill. The directory name doesn't have to match the skill name (the name comes from the frontmatter), but matching them keeps things easy to find.

## SKILL.md

YAML frontmatter + Markdown body:

```markdown
---
name: code-review
description: Run a structured code review on a diff. Looks at correctness, readability, test coverage, and security.
---

When invoked, ask the user for the diff if not provided. For each file in the diff:

1. Identify the **intent** — what is this change trying to accomplish?
2. Walk through the change and look for:
   - Correctness bugs
   - Readability issues
   - Missing tests
   - Security concerns (injection, auth bypass, secret handling)
3. Group findings by severity: **must fix** / **should fix** / **nice to have**.

Always cite line numbers when referring to code. End with a summary verdict.
```

| Field         | Required | Notes                                                                |
|---------------|----------|----------------------------------------------------------------------|
| `name`        | yes      | Used as the tool name the agent calls. Lowercase, dashes OK.         |
| `description` | yes      | Shown in `/skills` and used by the model to decide when to invoke.   |

The body is the prompt the agent sees when it invokes the skill.

## Lazy loading

Skills are discovered at startup but their bodies aren't read until first invocation — large skill bundles don't slow cold-start.

## `/skills` output

```
Skills:
  code-review — Run a structured code review on a diff…
  git-flow — Walk through our team's branch + PR conventions.
  deploy-checklist — Pre-deploy sanity check for production releases.
```

## Permissions

Skill invocations go through the permission gate using the `skills:` namespace:

```json
{
  "permissions": {
    "allow": ["skills:code-review", "skills:git-flow"]
  }
}
```

Most skills are read-only thinkers (no side effects), but the gate is still applied so a malicious or buggy skill can't bypass user approval.

## Discovery scope

Currently project-only — Cogo reads `.agents/skills/` from the project root. User-global skills (`~/.cogo/skills/`) are tracked as future work.

## Reload

`/reload` re-discovers skills. Add a new SKILL.md, run `/reload`, and the new skill is immediately available.
