---
title: Demostración de Sincronización Markdown
slug: markdown-sync-demo
path: /es/blog/markdown-sync-demo
summary: Aprende cómo go-cms importa contenido Markdown al iniciar.
status: published
tags:
  - markdown
  - cms
author: Equipo Demo
date: 2024-03-01T10:00:00Z
---

# Demostración de Sincronización Markdown

Este artículo reside en `examples/web/content/es/blog/markdown-sync.md` y
muestra cómo el servicio de Markdown de go-cms convierte archivos locales en
contenido del CMS.

Al iniciar la aplicación de ejemplo se ejecuta `Sync`, importando o actualizando
los registros existentes. Además, el comando de sincronización se registra en un
cron para que pueda ejecutarse de forma automática sin intervención manual.

Si editas este archivo y reinicias la aplicación observarás que el contenido se
actualiza; si lo eliminas y habilitas la opción `DeleteOrphaned`, el siguiente
cron eliminará el contenido huérfano.
