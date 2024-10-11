# Contributing to the k0s documentation

We use [mkdocs](https://www.mkdocs.org) and [mike](https://github.com/jimporter/mike) for publishing docs to [docs.k0sproject.io](https://docs.k0sproject.io).
This guide will provide a simple how-to on how to configure and deploy newly added docs to our website.

## Requirements

Install mike: https://github.com/jimporter/mike#installation

## Adding A New link to the Navigation

- All docs must live under the `docs` directory (I.E., changes to the main `README.md` are not reflected in the website).
- Add a new link under `nav` in the main [mkdocs.yml](https://github.com/k0sproject/k0s/blob/main/mkdocs.yml) file:

```yaml
# ... other directives
{%
    include "../../mkdocs.yml"
    start="# ~~~ START NAV SNIPPET ~~~\n"
    end="\n# ~~~ END NAV SNIPPET ~~~\n"
%}
  # more navigation links ...
```

- Once your changes are pushed to `main`, the "Publish Docs" jos will start running: https://github.com/k0sproject/k0s/actions?query=workflow%3A%22Publish+docs+via+GitHub+Pages%22
- You should see the deployment outcome in the `gh-pages` deployment page: https://github.com/k0sproject/k0s/deployments/activity_log?environment=github-pages

## Testing docs locally

We've got a dockerized setup for easily testing docs locally. Simply run
`make docs-serve-dev`. The docs will be available on http://localhost:8000.

**Note** If you have something already running locally on port `8000` you can
choose another port like so: `make docs-serve-dev DOCS_DEV_PORT=9999`. The docs
will then be available on http://localhost:9999.
