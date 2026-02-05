# Workflow Engine Demo

This example demonstrates the workflow engine capabilities in go-cms. A page is created in **draft** status and can be transitioned through various workflow states.

## Overview

The workflow engine provides state management for content entities like pages. The default page workflow supports these states:

- **draft** - Initial state for new content
- **review** - Content submitted for editorial review
- **approved** - Content approved and ready to publish
- **scheduled** - Content scheduled for future publication
- **published** - Content is live and visible
- **archived** - Content is archived and hidden

## Demo Setup

The example creates a page called "workflow-demo" with:
- **Initial Status**: `draft`
- **Slug**: `workflow-demo`
- **Content**: Demonstrates workflow transitions
- **Translations**: Available in English and Spanish

## API Endpoints

### 1. Set the Page ID

```bash
PAGE_ID=$(curl -s http://localhost:3000/api/pages | jq -r '.[] | select(.slug=="workflow-demo") | .id')
echo "Page ID: $PAGE_ID"
```

This automatically finds the workflow-demo page and stores its ID in the `PAGE_ID` variable.

### 2. Get Available Transitions

```bash
curl -s http://localhost:3000/api/pages/$PAGE_ID/transitions | jq '.'
```

This returns:
- Current page status
- List of available transitions from the current state

Example response:
```json
{
  "page_id": "...",
  "slug": "workflow-demo",
  "status": "draft",
  "transitions": [
    {
      "name": "submit_review",
      "from": "draft",
      "to": "review"
    },
    {
      "name": "publish",
      "from": "draft",
      "to": "published"
    },
    {
      "name": "schedule",
      "from": "draft",
      "to": "scheduled"
    },
    {
      "name": "archive",
      "from": "draft",
      "to": "archived"
    }
  ]
}
```

### 3. Transition Page to Published

There are two ways to transition a page:

**Option A: Using transition name**
```bash
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "publish"}' | jq '.'
```

**Option B: Using target state**
```bash
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"target_state": "published"}' | jq '.'
```

Both methods will transition the page from `draft` to `published`.

Example response:
```json
{
  "success": true,
  "page_id": "...",
  "slug": "workflow-demo",
  "from_state": "draft",
  "to_state": "published",
  "transition": "publish",
  "status": "published"
}
```

### 4. Verify Status Change

```bash
curl -s http://localhost:3000/api/pages/$PAGE_ID | jq '.status'
```

This should now return `"published"`.

## Example Workflow Scenarios

### Scenario 1: Direct Publish (draft → published)
```bash
# Set page ID
PAGE_ID=$(curl -s http://localhost:3000/api/pages | jq -r '.[] | select(.slug=="workflow-demo") | .id')
echo "Page ID: $PAGE_ID"

# Publish directly
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "publish"}' | jq '.'
```

### Scenario 2: Editorial Review (draft → review → approved → published)
```bash
# Set page ID
PAGE_ID=$(curl -s http://localhost:3000/api/pages | jq -r '.[] | select(.slug=="workflow-demo") | .id')
echo "Page ID: $PAGE_ID"

# Step 1: Submit for review
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "submit_review"}' | jq '.'

# Step 2: Approve
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "approve"}' | jq '.'

# Step 3: Publish
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "publish"}' | jq '.'
```

### Scenario 3: Unpublish and Archive
```bash
# Set page ID
PAGE_ID=$(curl -s http://localhost:3000/api/pages | jq -r '.[] | select(.slug=="workflow-demo") | .id')
echo "Page ID: $PAGE_ID"

# Unpublish (published → draft)
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "unpublish"}' | jq '.'

# Archive (draft → archived)
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "archive"}' | jq '.'
```

### Scenario 4: Other Workflow Transitions
**Important**: If the page is currently published, you must unpublish it first before transitioning to other states.

```bash
# Set page ID
PAGE_ID=$(curl -s http://localhost:3000/api/pages | jq -r '.[] | select(.slug=="workflow-demo") | .id')
echo "Page ID: $PAGE_ID"

# Step 1: Unpublish (published → draft)
# This step is required if the page is in published state
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "unpublish"}' | jq '.'

# Step 2: Submit for review (draft → review)
curl -sX POST http://localhost:3000/api/pages/$PAGE_ID/transition \
  -H "Content-Type: application/json" \
  -d '{"transition": "submit_review"}' | jq '.'

# Step 3: Check available transitions from review state
curl -s http://localhost:3000/api/pages/$PAGE_ID/transitions | jq '.'
```

## Viewing the Page

Once published, you can view the page at:
- English: http://localhost:3000/blog/workflow-demo
- Spanish: http://localhost:3000/blog/workflow-demo?locale=es

**Note**: Draft pages are typically not visible on the frontend. Only published pages should be rendered.

## Workflow Engine Details

The workflow engine is implemented in `internal/workflow/simple/engine.go` and provides:
- State transition validation
- Available transition queries
- Extensible workflow definitions
- Support for custom entity types

See the [workflow documentation](../../docs/) for more details on:
- Creating custom workflows
- Adding workflow guards/permissions
- Event emission during transitions
- Scheduled publication
