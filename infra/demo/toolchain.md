# W0 development toolchain

[toolchain.json](toolchain.json) and [requirements-dev.lock](requirements-dev.lock)
pin this baseline. Package metadata was checked against official Go/PyPI
sources on 2026-09-25; pins are deliberate, not an automatic latest-version
policy. No runtime service or provider module is introduced by these pins.

| Tool | Selection / lock / verification |
| --- | --- |
| Rust | 1.94.0 at this baseline; existing `feather-mesh/Cargo.lock`. MAC all-feature tests and Clippy use the existing installation. The native Ubuntu workflow selects 1.94.0 explicitly. |
| Python | 3.13 control/CI; Ubuntu system Python 3.12 for read-only probes; tools in an isolated venv. All Python tooling dependencies are exact-version locked. |
| Go | 1.27.1, official darwin/arm64 and linux/amd64 archive SHA-256 in the JSON lock. MAC archive installed and `go version` verified. No module dependencies yet; introduce `go.mod`/`go.sum` with W1 service source. |
| Ansible | core 2.21.4; ansible-lint 26.9.0 plus transitive pins. Uses built-in modules only, no unpinned Galaxy collections. Local syntax/lint checks require no target credentials. |
| Settings schemas | jsonschema 4.26.0, Draft 2020-12; validation never resolves remote `$ref` URLs. All current schemas are self-contained. |
| Browser tests | Playwright Python 1.63.0 selected/installed; Chromium binaries and real browser tests arrive with W1 proxy. No `npm` dependencies or browser claim at W0. Install the browser matching this package, not a globally managed browser. |
| Terraform | 1.14.5 selected; installation and provider locks deferred until W7/first Terraform module. No Terraform source exists to validate in W0. Fetch the official archive/checksum and create `.terraform.lock.hcl` before introducing a module. |
| Linux full VM | Ubuntu 24.04 x86-64 cloud image release 20260911; signed checksum checked. QEMU 8.2.2 and 15 supporting packages are pinned by version/hash in `scripts/ubuntu-vm-packages.lock.json`. The authorized TCG guest passed W0 systemd/cgroup/loop tests; this supplies functional, not performance evidence. |

The existing Mac Node/npm are not required by W0. No global package manager,
system Python, Docker daemon or shell profile is modified. Temporary tools can
be removed after the session; recreate them from the lock for another checkout.
Go archive checksums were taken from the [official download metadata](https://go.dev/dl/?mode=json).
[Ansible support policy](https://docs.ansible.com/projects/ansible/latest/reference_appendices/release_and_maintenance.html)
and [Playwright browser installation](https://playwright.dev/python/docs/browsers)
describe controller and browser constraints. Terraform's selected archive is
listed in [official releases](https://releases.hashicorp.com/terraform/1.14.5/).

Native dependency policy: preserve current Rust lock/feature checks and build
`--locked` on Ubuntu. Use Ubuntu-supported native libraries plus pinned Python
wheels for the image's SDK/Polars/Rasterio stack; record actual wheel/library
versions, architecture and artifact hashes at W1. Existing requirements that
allow ranges are not a future immutable image manifest. Generate fixtures and
run real readers inside that exact image before release. No macOS binary is
copied into a Linux image. Keep images, seeds and model profiles independently
versioned. The local preflight found `uidmap` missing; installing it belongs to
reviewed W1 bootstrap, not W0 inspection.
