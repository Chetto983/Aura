# The stack comes back with Docker; the wizard can see an existing install

Date: 2026-10-15, lab VM 192.168.101.158. This file closes the two items the 2026-10-14 evening E2E
left open ([2026-10-14-gitlab-image-bridge.md](2026-10-14-gitlab-image-bridge.md)):
- stopping Docker took the stack down, and nothing brought it back;
- the wizard never showed "existing installation detected".

## What changed

| Commit | Change |
|---|---|
| 35864cc2f | `aura.service` is `WantedBy=multi-user.target docker.service`, so every Docker start runs the ordered `compose up` again. |
| fac26edd4 | `apply_update` re-enables every enabled unit that lacks a link its `WantedBy=` names. This replaces 35864cc2f's "re-enable what this tick replaced"; see "Found on the way". |
| 42b755fe3 | `install.sh` sets INSTALL_DIR to 0755 on every run, inside the new `create_install_dirs`. |
| 35c7dc457 | The makeself artifact is built with `--nox11`. |
| fc472d3b8, 37e9e9a61 | The payload manifest follows the unit and the updater. |
| 032f8e0f0 | create-aura 0.2.15. |

## E2E 1: Docker stop/start and restart, with the unit installed by hand

The new unit was installed the way an updater installs a changed unit: copy, `daemon-reload`, then
`reenable`, because the unit was enabled. That created
`/etc/systemd/system/docker.service.wants/aura.service`. The update timer was held for the run and
restored afterwards.

| Step | Result |
|---|---|
| `systemctl stop docker.socket docker.service` | Docker inactive, unit inactive, 0 containers |
| `systemctl start docker.socket docker.service` | Whole and healthy after **75 s**: 17/17 compose containers, unit active |
| `systemctl restart docker.service` | Whole and healthy after **87 s**: 17/17, unit active |

## E2E 2: a host as it was before today, taking one real update tick

**The starting state** was what an existing host such as Andrea's has:
- the pre-35864cc2f unit (`WantedBy=multi-user.target`);
- no `docker.service.wants` link;
- last night's updater, whose source has no `relink_enabled_units` (0 matches);
- aura on 3e15e058c.

**The tick.** It went to the edge published from 37e9e9a61, through
`systemctl start aura-image-update.service`: rc 0 in 167 s, Result `success`. Journal, in order:
- the old updater installed `deploy/aura-image-update.sh`, `deploy/aura.service`,
  `/usr/local/sbin/aura-image-update.sh` and `/etc/systemd/system/aura.service`;
- `payload: the updater changed; re-executing it.`
- **`units: aura.service re-enabled for docker.service.`** This line comes from the new updater:
  the old one copied the unit and never re-enabled it;
- aura moved to a new image digest, and the stack was brought up on the installed payload;
- `Aura image update completed; the appliance is healthy.`

**After the tick:**
- `WantedBy=multi-user.target docker.service`;
- the docker.service link present;
- the installed updater has `relink_enabled_units`;
- aura on 37e9e9a61, healthy.

**Then `systemctl restart docker.service`:** whole and healthy after **100 s**, 17/17, unit active.
The timer was restored to active.

## Found on the way

**35864cc2f alone would not have reached any existing host.** On a host, the new unit is copied by
the updater being replaced, which predates any re-enable. The re-executed updater then finds the
unit current, and 35864cc2f re-enabled only what the running tick replaced. E2E 1 passed only
because the link had been made by hand. fac26edd4 checks the links instead of who copied the file,
and E2E 2 is that path, run for real.

**The makeself artifact spawned an X11 terminal.**
- **Trigger:** with no tty and a `DISPLAY` set, where `xset q` answers, makeself's default header
  execs an X11 terminal emulator.
- **Effect:** on a Linux desktop the wizard's spawned installer would open a separate terminal
  window. Under WSLg, with no emulator installed, the header died on `exec: -e: not found` before
  `install.sh` ran. `build_installer_test.sh` failed that way locally, and on HEAD too.
- **Why CI missed it:** CI has no DISPLAY.
- **Fix:** 35c7dc457 builds with `--nox11`, and the test stubs a desktop session, so the failure
  shows on any machine.

**The first publish failed.** The 032f8e0f0 publish failed at `git clone` of the whatsapp-mcp fork
(`curl 56`, connection timed out). Forks are cloned before any push, so nothing was published. The
publish from 37e9e9a61 succeeded:
- every edge tag moved to it under `registry.gitlab.com/chetto1983/aura`;
- no local tags were left behind.

## create-aura 0.2.15

The operator published it at 09:55:37Z.

**The registry's `gitHead` is 032f8e0f0.** The artifact was packed at 11:54:20, 18 s before
fac26edd4 was committed, from a working tree that already carried that change:
- **The local package matches the registry:** 63 files, 213,159 bytes, against `dist.fileCount`
  63 and `dist.unpackedSize` 213159.
- **The artifact's payload, extracted from the archive without running it:**
  - its `deploy/aura-image-update.sh` is blob b40e3b6d, the same as fac26edd4 and 37e9e9a61. The
    032f8e0f0 updater is 606 bytes shorter, so it could not give the same total;
  - its `deploy/aura.service` and `install.sh` are HEAD's;
  - its header carries `nox11=y`.

## Local harness notes (WSL)

- **`install_remote_access_test.sh`** waits forever on a sudo password prompt when not run as
  root, and so does `build_installer_test.sh`, which calls it. That is true on HEAD as well. Run
  either with `wsl -u root`. As root both pass on this tree.
- **The other suites** pass as the login user: `install_lib_test.sh` (including the new umask-077
  case), `aura_image_update_test.sh`, `install_config_test.sh`, `install_preflight_test.sh`,
  `aura_update_consent_test.sh`, `appliance_posture_test.sh` and `go test ./cmd/aura/`.

## Not shown here

- **The wizard notice itself, end to end.** That needs two npx runs of 0.2.15 on one host. The fix
  is shown only by the umask-077 test and the extracted artifact. A host installed before today
  shows the notice from its second re-run on.
- **Andrea's box.** It is still frozen behind the bootstrap patch. E2E 2 reproduces its starting
  state, not the box itself.
- **A Docker package upgrade** that restarts the daemon through apt. `systemctl restart` stands in
  for it here.
- **arm64.**
