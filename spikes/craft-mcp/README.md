# Spike: the Craft MCP servers mounted in Aura's box

Date: 2026-10-18. Throwaway. Asked by the operator: "prima monta tu gli MCP nativi e valutali",
before deciding whether LibreOffice and the Python Office libraries leave the box and the agent
keeps only the Craft path.

The 2026-10-18 survey (`D:/tmp/storytold-survey.md`) ran each `<app>-cli mcp` with `docker run`
on `aura-sandbox:local`. It did not go through Aura: not `aura mcp add --box`, not the identity's
box, not the bridge, `tool_search` or the image sink, and not the agent. This spike does.

## The servers

| Server | Command in the box |
|---|---|
| gridcraft | `gridcraft-cli mcp` (no confinement flags exist) |
| pdfcraft | `pdfcraft-cli mcp --compact --automation-read-root /workspace --automation-write-root /workspace` |
| wordcraft | `wordcraft-cli mcp --automation-read-root /workspace --automation-write-root /workspace` |
| deckcraft | `deckcraft-cli mcp --automation-read-root /workspace --automation-write-root /workspace` |
| filmcraft | `filmcraft-cli mcp` (no confinement flags exist) |
| photocraft | `photocraft-cli mcp --automation-read-root /workspace --automation-write-root /workspace` |

## Questions

1. **The mount.** Does each server pass `aura mcp add --box` and reach the agent as deferred
   tools that `tool_search` finds? What do its image results become?
2. **The work.** On files from the operator's library, does a realistic task per server produce
   an output an independent tool confirms? How long does it take, and how much memory?
3. **Today's path.** The same task through what the box does today (LibreOffice, openpyxl,
   python-docx, python-pptx, pypdf, qpdf, tesseract): what is equal, better or worse?
4. **Confinement in the real box.** Do the roots hold? What can the two servers without roots
   reach?
5. **The agent.** With one turn per server, does the real agent find the tools and finish the task?
6. **What removal would cost.** What does the agent do today with LibreOffice and the Python
   libraries that no Craft server does?

## Method

- `Dockerfile`: the box image plus the six CLIs, checked against their digests. Tagged over
  `aura-sandbox:local` for the run, after the original is kept as `aura-sandbox:pre-craft`.
- Each server registered as the operator would: `docker exec aura aura mcp add <name> --box -- …`.
- Questions 2–4 are driven without the model; question 5 spends one agent turn per server (the
  model is local, and a turn holds the GPU).

## Findings

See `FINDINGS.md`.
