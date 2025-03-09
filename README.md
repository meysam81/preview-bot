# GitHub PR Preview Bot

A lightweight tool that automatically comments on GitHub Pull Requests with deployment preview links.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Overview](#overview)
- [Quickstart](#quickstart)
  - [Running locally](#running-locally)
  - [Kubernetes Integration Example](#kubernetes-integration-example)
- [Environment Variables](#environment-variables)
- [Template](#template)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Overview

This project provides a simple Go application that:

- Posts comments on GitHub PRs with preview deployment URLs
- Removes previous preview comments when updated
- Uses customizable templates for comment formatting

[![Preview Bot Comment](./assets/preview-bot-comment.png)](https://github.com/meysam81/preview-bot)

Designed to run within Kuberntes Pods to provide stakeholders with easy access
to deployment previews for each PR.

## Quickstart

### Running locally

```bash
# Set required environment variables
export PR_NUMBER="123"
export USER_LOGIN="github-user"
export COMMIT_SHA="abc123def456"
export URL="https://preview-domain.example.com"
export GITHUB_TOKEN="ghp_yourtokenhere"

# Run the application
go run main.go your-org/your-repo
```

### Kubernetes Integration Example

```yaml
---
apiVersion: batch/v1
kind: Job
metadata:
  name: some-important-task
spec:
  template:
    spec:
      containers:
        - command:
            - sh
            - "-c"
            - echo hello world
          image: busybox:1
          name: busybox
          resources: {}
      initContainers:
        - args:
            # repo in the format owner/repo
            - meysam81/preview-bot
          env:
            # github username
            - name: USER_LOGIN
              value: meysam81
            - name: GITHUB_TOKEN
              value: ghp_yourtokenhere
            - name: COMMIT_SHA
              value: abc123def456
            - name: PR_NUMBER
              value: "123"
            - name: URL
              value: https://pr123.example.com
          image: ghcr.io/meysam81/preview-bot
          name: preview-bot
          resources:
            limits:
              cpu: 10m
              memory: 10Mi
            requests:
              cpu: 10m
              memory: 10Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
            runAsGroup: 65534
            runAsNonRoot: true
            runAsUser: 65534
          terminationMessagePolicy: FallbackToLogsOnError
      restartPolicy: OnFailure
```

## Environment Variables

| Variable       | Description                            | Default                |
| -------------- | -------------------------------------- | ---------------------- |
| `PR_NUMBER`    | Pull request number                    | Required               |
| `USER_LOGIN`   | GitHub username for the bot            | Required               |
| `COMMIT_SHA`   | Commit SHA being deployed              | Required               |
| `URL`          | Deployment preview URL                 | Required               |
| `GITHUB_TOKEN` | GitHub API token with repo permissions | Required               |
| `TITLE`        | Comment title                          | `# Preview Deployment` |
| `ASSETS_DIR`   | Directory for template files           | `/`                    |
| `DEBUG`        | Enable debug logging                   | `false`                |

## Template

Create a file named `preview-body.md.tpl` in your assets directory with available variables:

- `{{TITLE}}`: The comment title
- `{{COMMIT_SHA}}`: The commit SHA
- `{{URL}}`: The deployment URL
