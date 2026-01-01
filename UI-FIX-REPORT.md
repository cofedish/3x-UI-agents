# Servers Page Grid Spacing Fix

## Problem
Negative margins (`margin: -6px -8px`) on inner rows inside card caused content to stick to edges.

## Root Cause
```html
<!-- BEFORE (servers.html) -->
<a-card :style="{ padding: '16px' }">
  <a-row :gutter="12">  <!-- ← generates margin: -6px -8px -->
    ...
  </a-row>
</a-row>
```

Ant Design automatically generates negative margins when `gutter` is set:
- `gutter="12"` → `margin: -6px -6px` (12/2 = 6)
- Combined with card padding, creates visual issues

## Baseline Pattern (inbounds.html)
```html
<!-- CORRECT (inbounds.html) -->
<a-card :style="{ padding: '16px' }">
  <a-row>  <!-- ← NO gutter, no negative margins -->
    <a-col :sm="12" :md="5">
      ...
    </a-col>
  </a-row>
</a-card>
```

Inner rows don't need gutter when already inside a padded container.

## Solution
Removed `:gutter="12"` from both inner rows:
- Filters row (line 39)
- Stats row (line 78)

## Changes
**File**: `web/html/servers.html`
- Line 39: `<a-row :gutter="12" :style="...">` → `<a-row :style="...">`
- Line 78: `<a-row :gutter="12" :style="...">` → `<a-row :style="...">`

## Verification Checklist
- [x] No negative margins on inner rows
- [x] Content doesn't stick to card edges
- [x] Spacing matches inbounds page
- [x] Filters row displays correctly
- [x] Stats row displays correctly
- [x] Table renders properly
- [x] Light/dark themes work

## Before/After
**Before**: `<div class="ant-row" style="margin: -6px -8px;">`
**After**: `<div class="ant-row" style="margin-bottom: 10px;">`

Outer row (line 35) keeps `gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]"` for proper layout-level spacing.
