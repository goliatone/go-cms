---
title: Markdown Sync Demo
slug: markdown-sync-demo
path: /blog/markdown-sync-demo
summary: Learn how go-cms imports Markdown content during startup.
status: published
tags:
  - markdown
  - cms
author: Demo Team
date: 2024-03-01T10:00:00Z
---

# Markdown Sync Demo

Welcome to the markdown-powered blog post. This file lives under
`examples/web/content/en/blog/markdown-sync.md` and demonstrates how file-based
content can flow into go-cms automatically.

On application startup the example calls the Markdown service's `Sync` method,
creating or updating CMS content based on the files in the content directory.
The scheduled command registration hooks into go-command so the same sync can
run periodically through a cron expression.

Updating this file and rerunning the example will mark the content as updated,
while removing it and enabling the `DeleteOrphaned` flag would prune the CMS
record during the next sync run.
