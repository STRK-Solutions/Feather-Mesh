# Feather Mesh documentation site

This is the initial static documentation site for the development demo. Its
Markdown sources are also readable directly by coding assistants; the
**Agent context** page is a compact orientation, not a replacement for the
repository's contracts or implementation.

The site is built automatically on pull requests and pushes to `main` by
[the documentation workflow](../.github/workflows/docs-site.yml). The workflow
uploads the generated static site as a build artifact. It does not deploy or
publish a hosted service; integration with the planned Ubuntu demo remains a
separate workplan task.

## Preview locally

From the repository root, create a virtual environment and install the pinned
documentation dependencies:

```powershell
py -m venv .venv-docs
.\.venv-docs\Scripts\python -m pip install -r demo_site\requirements.txt
.\.venv-docs\Scripts\python -m mkdocs serve --config-file demo_site\mkdocs.yml
```

On macOS or Linux:

```sh
python3 -m venv .venv-docs
.venv-docs/bin/python -m pip install -r demo_site/requirements.txt
.venv-docs/bin/python -m mkdocs serve --config-file demo_site/mkdocs.yml
```

Open the local address printed by MkDocs. To produce the same static bundle as
CI, replace `serve` with `build --strict`.
