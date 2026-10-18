# Studio

The cockpit's creative workspace, per identity.

- **Generate images and video** from a prompt over OpenRouter's media models. The model
  picker shows each model's price (per image, per second or per million output tokens)
  and the estimated cost before you press Generate. Options cover resolution, aspect
  ratio, seed, and duration and sound for video; advanced inputs take a start frame, an
  end frame and reference images from your library. Every generation lands in a
  searchable history you can reuse or download. Generation needs the OpenRouter route.

Editing is the agent's: photos through PhotoCraft and video through FilmCraft, two of the Craft
MCP servers that run in the identity's box (docs/superpowers/specs/2026-10-18-craft-mcp-design.md).
The browser editors the Studio had before were removed with that change.
