# Verification — gVisor retired, the lab VM after its update

Run on 2026-10-11 on the lab VM `192.168.101.158`, for the last step of
`docs/superpowers/plans/2026-10-11-retire-gvisor.md`: the appliance converges to the gVisor-free
payload through its own updater, with no manual step. Every command was a read (`docker
inspect`, `docker ps`, `grep`, `jq`, `journalctl`, one `curl` to `/healthz`).

## Before the update (13:43 UTC)

- `aura` ran revision `36a6baa4d` (the `installer-v0.2.10` commit), started at 11:56.
- `/opt/aura/compose.yaml` still carried `runtime: ${AURA_RUNTIME:-runc}` on the `aura` service,
  and the installed `/etc/systemd/system/aura.service` mentioned gVisor 9 times (its comment block
  on installing `runsc`).
- `AURA_RUNTIME` was absent from `/opt/aura/.env`, and every container already ran on `runc`, as
  the browser-sidecar ledger of the same morning had measured.

## After the update

`081fba106` was pushed after that read. Its `:edge` image was published, and the updater's timer
pulled it and applied the payload at 13:51 with no operator action; `aura` restarted at
13:51:52Z on revision `081fba106`.

| Check | Result |
|---|---|
| `runtime:` lines in `/opt/aura/compose.yaml` | 0 |
| gVisor or `runsc` mentions in the installed `aura.service` | 0: the updater syncs the unit too (`deploy/aura-image-update.sh`) |
| `HostConfig.Runtime` of every container, 22 of them, the operator's box and its egress sidecar included | `runc` |
| `AURA_RUNTIME` in `/opt/aura/.env` | absent |
| `/etc/docker/daemon.json` | `runtimes` still lists `runsc`, `default-runtime` unset: left inert on purpose (plan, Global Constraints) |
| Health at 13:54 | 16 healthy, 0 unhealthy, 0 starting; the containers without a healthcheck up; `garage-bootstrap`, `migrate` and `tempo-blocks-repair` exited 0; `https://127.0.0.1/healthz` through caddy returned 200 |

The caddy, ingest and cloudflared restart came about 15 seconds after `aura`, and they were still
`starting` at 13:52; the 13:54 read found them healthy.

## After this run

At 14:21 UTC GitHub flagged the `chetto1983` account ("owner marked spammy" on every cancelled
Actions job). From 14:22:49 the VM's updater fails with `error from registry: denied`, and an
anonymous pull of `ghcr.io/chetto1983/aura:edge` returns 403. The stack measured above keeps
running on `081fba106`; the release commit `64ec61820`, which changes only the wizard's version,
never reached it. The VM boots with `AURA_PULL_POLICY=always`, so while the flag stands a restart of
`aura.service` would likely fail to pull; that was not tried.

## What this does not prove

- **The runsc recreate path.** No box on this VM had ever run on gVisor, so `staleBoxReason`
  returning "the retired gVisor runtime" is proven by `TestStaleBoxReason` only, not on a live
  daemon.
- **A host that had `AURA_RUNTIME=runsc` in its `.env`.** This VM never set it. Compose no longer
  reads the variable, so the `aura` container falls to the daemon default; that follows from the
  compose file, not from a measurement.
- **A fresh install.** The installer was not run here. Its gVisor-free behaviour is covered by
  `scripts/install_config_test.sh` and `scripts/install_lib_test.sh`, not by an install on a clean
  machine.
- **Removing `runsc`.** Nothing removes it, by the operator's choice; a host that wants it gone
  does so by hand.
