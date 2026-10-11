# Browser-sidecar lab deployment — 2026-10-11

The operator requested immediate deployment to 192.168.101.158 and will run the
acceptance tests himself. No additional agent E2E turns were run. Mutation
testing is explicitly forbidden by the operator and was stopped; its affected
source file was restored and byte-verified against the pre-run backup.

## Deployed

- Aura image: `aura:lab-browser-sidecar-20261011`,
  `sha256:46b2578318fc7786169a15d197a6dfc432b2f27897bbc8f678bbd801e12659d3`.
- Linux binary SHA-256:
  `7d36f0f9897807bab12cd216201d1374ce8b1a34cdab395ae3c494d4b457a645`.
- Sandbox image: `aura-sandbox:lab-browser-sidecar-20261011`,
  `sha256:35d7a347803c04e10cca7581ca01c51e261ce44f60975f4840debdd3a0f8d3a7`.
- Overlay: `/opt/aura/lab-browser-sidecar-20261011-01/compose.lab.yaml`,
  applied after `/opt/aura/compose.yaml` and `compose.cpu.yaml`, using
  `up -d --no-deps --pull never aura`.
- The old operator shell and egress containers were stopped, not purged.
  Ordinary workspaces and browser profiles were not manually removed.
- Browser containers remain lazy: the first routed tool/live-view call performs
  verified profile migration before returning a shell or browser handle.
- LLM provider and OAuth configuration were not changed.
- No git commit, push or registry publication was performed.

The Aura child image replaces only its binary. The sandbox child image replaces
only the browser entrypoint. Its final image was made from a fresh, never-started
container without identity mounts or credentials, not from the operator box.

## Deployment checks, not acceptance

Aura reports `running` / `healthy`; the frontend `/login` returns HTTP 200.
The unauthenticated protected root returns HTTP 401. The deployed binary hash
and configured sandbox image were checked directly. Disk space recovered to
6.7 GiB available after temporary build work settled; no user volumes or
unrelated images were pruned. Only this deployment's identified build-cache
records were targeted for cleanup.

`aura-image-update.timer` was active beforehand and is temporarily stopped so
the edge updater does not overwrite this local build during operator testing.
Its original state and base image IDs are recorded in the lab directory.
Do not restart the updater before deciding how the local build will be replaced:
it uses the base compose files, not this overlay.

## Not demonstrated by this deployment

- Real-agent E2E, browser live view, actual download crossing, admission and
  idle behavior on this deployed VM are left to the operator.
- The account supplied by the operator is the Aura frontend account, not a
  Google browser login. Frontend sign-in passed before deployment; no
  authenticated Google session was established or claimed as preserved.
- Migration of the existing profiles has not been forced via a test turn.
- Full acceptance, independent review, package-local coverage closure, commits,
  push and green CI are not claimed.
