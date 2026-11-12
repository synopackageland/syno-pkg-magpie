# Magpie for DSM

A native DSM package that runs the upstream [magpie](https://github.com/yetone/magpie)
web panel behind DSM administrator sign-in, and magpie's own model gateway on
its native port for the rest of the LAN.

- Upstream: `https://github.com/yetone/magpie` (MIT), version `0.1.3093`
- Package: `Magpie`, DSM 7.2.1-69057 or later, `x86_64` and `aarch64`
- Unsigned SPK: install with Package Center → Manual Install
- Runs as the package account, no containers and no root

This directory is the whole wrapper. Every file here is packaging; the upstream
tree stays byte-identical and is only compiled. Package Center distribution is
by Syno Package Land; the upstream project is the maintainer.

## Current rebuild

Package 0.1.3093-0009 uses SDK/App 0.53.21 and CLI 0.53.16 to improve
installation reliability in Package Center: the gateway nginx reload is declared
only for start and stop, not for install or uninstall. The upstream version,
daemon, authentication policy and updater behavior are unchanged. Validation is
limited to check, build, baseline inspect and leak scans; tests and NAS checks
were not run.

## What runs

```text
Package Center Open / DSM main menu icon
        │
        ▼
ui/sso/open.cgi  ──►  https://<NAS>:<gateway port>/        (DSM nginx, DSM certificate)
        │                 │ DSM session + administrator check
        │                 ▼
        │              daemon/payload/_sso/gateway          (SDK identity gateway, token mode)
        │                 │ injects Cookie: magpie_web_3430=<panel key>
        │                 ▼
        │              127.0.0.1:3430                       (magpie's own web panel)
        │
        └── magpie-launch execs magpie serve on 0.0.0.0:3425 (model gateway, magpie gateway keys)
```

- **Management UI** — magpie's panel (`magpie serve --web=127.0.0.1:3430`).
  DSM administrators only (`entry: admin-only`, `access: dsm-admins`). The
  gateway supplies magpie's own panel key server-side, so there is no second App
  password and the browser never receives the key.
- **Advertised address** — `magpie-launch` (in the payload) derives
  `MAGPIE_PUBLIC_URL` from this NAS right before magpie starts: the address of
  the interface on the default route, else the first real private address, never
  loopback, link-local, a Docker/VM bridge or a proxy fake-IP range. magpie's
  panel then lists that one address instead of every interface, on any NAS,
  with nothing hard-coded. An operator-set `MAGPIE_PUBLIC_URL` wins. The panel's
  "This computer" line stays `127.0.0.1`, which is what processes on the NAS
  itself use.
- **Plugin pages** — a community plugin's page (Chat, Agent Office, RSSHub…)
  is shown in a sandboxed frame. A sandbox makes the frame an opaque origin, so
  the browser withholds this App's `SameSite` cookies and the gateway would
  answer every asset with `dsm_sign_in_required`. `identity_gateway
  .upstream_auth_paths: ["/_magpie/plugin"]` hands exactly those paths to
  magpie's own per-run page token instead (CSP-sandboxed, `connect-src 'none'`,
  token only ever handed out by the signed-in `/api`). The panel, `/api` and the
  SDK routes still require a DSM administrator.
- **Model gateway** — `http://<NAS>:3425` serves the OpenAI Chat/Responses,
  Anthropic Messages and Gemini APIs. It is **not** behind DSM: clients use a
  magpie gateway key (Gateway → Gateway keys in the panel), exactly as magpie
  documents for another computer. magpie refuses a request from another machine
  without an enabled key, so create one before pointing agents at the NAS.
- The gateway's public HTTPS port is chosen in the install wizard (default
  `28443`). Port `3425` is magpie's own upstream contract and is not configurable.

## Install

1. Download the SPK that matches the NAS CPU
   (`x86_64` = Intel/AMD, `aarch64` = ARM64).
2. Package Center → **Manual Install** → choose the `.spk` file.
3. The SPK is unsigned, so DSM asks you to allow a package from an unknown
   publisher first: Package Center → Settings → General → Trust Level →
   **Any publisher**, then install again.
4. During install, pick the panel's HTTPS port and install.
5. Open **Magpie** from the DSM main menu (or Package Center → Open). It opens
   the panel in a new tab after DSM sign-in.

The first start creates magpie's own configuration under
`/var/packages/Magpie/var` (its `HOME`). That folder survives upgrades; the
uninstall wizard's delete-data checkbox removes it.

## Use

- **Add providers** in the panel (Providers → Add provider). Everything magpie
  supports upstream works the same here; magpie keeps its config in the package
  data folder.
- **Create a gateway key** in the panel (Gateway → Gateway keys) before using the
  model API from another machine:

  ```sh
  export OPENAI_BASE_URL=http://<NAS>:3425/v1      OPENAI_API_KEY=<gateway key>
  export ANTHROPIC_BASE_URL=http://<NAS>:3425      ANTHROPIC_API_KEY=<gateway key>
  export GOOGLE_GEMINI_BASE_URL=http://<NAS>:3425  GEMINI_API_KEY=<gateway key>
  ```

- DSM administrators all share the same magpie configuration: there is one
  upstream magpie, not one per DSM user.

## Limitations

- magpie's panel is upstream's web build. Its agents, CLIs and plugin
  dependencies are not installed by this SPK: pages that install an agent or run
  a plugin's Node/Bun work still need those tools on the NAS and may fail there.
- magpie keeps its own updater. It downloads an upstream release, replaces the
  payload binary in `target/daemon/payload/magpie` and re-execs itself; the
  daemon supervisor's digest covers `daemon/config.json`, not the payload, so
  the service keeps running. A later SPK upgrade replaces that binary again, so
  the packaged version is the floor, not a pin. The package account must be able
  to write the payload directory (DSM owns `target/` for the package); if it
  cannot, the update fails in the panel and the packaged binary stays. The
  installed SPK's own checksum and version then no longer describe the running
  binary — Package Center still shows the SPK version it installed.
- The panel key is a fixed value in `dsm-package.yaml`. It only ever reaches the
  panel from the package's own gateway over loopback; the panel does not listen
  on the LAN.
- Package start requires the DSM nginx `web-config` resource to own the gateway
  port. If the port was taken by another service, choose another one at install.

## Build and upgrade

```sh
bash synology/upgrade.sh v0.1.3093 2   # fetch/merge the tag, rebuild, check, build
synopkgland check --json
synopkgland build --json               # → synology/dist/Magpie-<version>-<build>-<arch>.spk
synopkgland inspect synology/dist/Magpie-0.1.3093-0002-x86_64.spk --json
```

`build-source.sh` compiles the merged upstream tree (`-tags nogui`, CGO off)
and `adapter/` (the gateway token adapter) into `daemon/<arch>/` for every
declared architecture. Needs Go 1.26.10 or later; the Go module sums come from
upstream's `go.mod`/`go.sum`.

Local checks (no NAS): `npm run test` (token adapter) and `npm run test:local`
(check, build and inspect one architecture). `npm install` is only needed for the
local tier.

## Attribution

magpie is `yetone/magpie`, MIT. The upstream license and third-party notices are
shipped in `LICENSES/magpie-MIT.txt` and
`LICENSES/magpie-THIRD-PARTY-NOTICES.txt`, and magpie embeds the same notices in
its binary.
