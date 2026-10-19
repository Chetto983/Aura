# Studio

The cockpit's creative workspace, per identity.

- **Generate images and video** from a prompt over OpenRouter's media models. The model
  picker shows each model's price (per image, per second or per million output tokens)
  and the estimated cost before you press Generate. Options cover resolution, aspect
  ratio, seed, and duration and sound for video; advanced inputs take a start frame, an
  end frame and reference images from your library. Every generation lands in a
  searchable history you can reuse or download. Generation needs the OpenRouter route.

Editing is the agent's, through the PhotoCraft and FilmCraft MCP servers in the identity's box
(docs/superpowers/specs/2026-10-18-craft-mcp-design.md), and the operator's, through the same apps'
web builds: in the file manager a photo or a clip has Edit, which opens it in PhotoCraft or
FilmCraft, and a save (FilmCraft: the project or an export) lands beside it in the library
(docs/superpowers/specs/2026-10-19-craft-web-apps-design.md). The browser editors the Studio had
before were removed with the first change.
