---
title: Authenticate
weight: 2
---

Cogo speaks Gemini through one of two auth paths. Pick whichever matches the credentials you have.

## Option A — Public Gemini API

Best for personal use and quick experiments.

1. Get an API key from [Google AI Studio](https://aistudio.google.com/apikey).
2. Export it:

```bash
export GOOGLE_API_KEY=...
```

That's it. Cogo will detect the key on startup and use the public Gemini endpoints.

## Option B — Vertex AI

Best for org-managed projects, longer context windows, and tighter audit trails. Requires a GCP project with the Vertex AI API enabled.

1. Authenticate with `gcloud`:

```bash
gcloud auth application-default login
```

2. Tell Cogo to use Vertex:

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=your-project
export GOOGLE_CLOUD_LOCATION=us-central1   # or "global"
```

The same Application Default Credentials (ADC) flow that other GCP tooling uses.

## Verify

```bash
cogo -p "ping"
# pong
```

If you see an error, double-check that exactly one auth path is configured (e.g. don't have both `GOOGLE_API_KEY` and `GOOGLE_GENAI_USE_VERTEXAI` set).

## A copy-pasteable template

The repo ships an `.env.example` you can drop in as a starting point — copy it to `.env`, edit the values, and `source .env` before running.

## Next

→ [First Prompt](../first-prompt/) — run a turn.
