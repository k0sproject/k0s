# Publishing Docs

We use [mkdocs](https://www.mkdocs.org) and [mike](https://github.com/jimporter/mike) for publishing docs to [docs.k0sproject.io](https://docs.k0sproject.io).
This guide will provide a simple how-to on how to configure and deploy newly added docs to our website.

## Requirements

Install mike: https://github.com/jimporter/mike#installation

## Adding A New link to the Navigation

- All docs must live under the `docs` directory (I.E., changes to the main `README.md` are not reflected in the website).
- Add a new link under `nav` in the main [mkdocs.yml](https://github.com/k0sproject/k0s/blob/main/mkdocs.yml) file:

```yaml
nav:
  - Overview: README.md
  - Creating A Cluster:
      - Quick Start Guide: create-cluster.md
      - Run in Docker: k0s-in-docker.md
      - Single node set-up: k0s-single-node.md
  - Configuration Reference:
      - Architecture: architecture.md
      - Networking: networking.md
      - Configuration Options: configuration.md
      - Using Cloud Providers: cloud-providers.md
      - Running k0s with Traefik: examples/traefik-ingress.md
      - Running k0s as a service: install.md
      - k0s CLI Help Pages: cli/k0s.md
  - Deploying Manifests: manifests.md
  - FAQ: FAQ.md
  - Troubleshooting: troubleshooting.md
  - Contributing:
      - Overview: contributors/overview.md
      - Workflow: contributors/github_workflow.md
      - Testing: contributors/testing.md
```

- Once your changes are pushed to `main`, the "Publish Docs" jos will start running: https://github.com/k0sproject/k0s/actions?query=workflow%3A%22Publish+docs+via+GitHub+Pages%22
- You should see the deployment outcome in the `gh-pages` deployment page: https://github.com/k0sproject/k0s/deployments/activity_log?environment=github-pages

## Testing docs locally

We've got a dockerized setup for easily testing docs in local environment. Simply run `docker-compose up` in the docs root folder. The docs will be available on `localhost:80`.

**Note** If you have something already running locally on port `80` you need to change the mapped port on the `docker-compose.yml` file.