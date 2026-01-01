# Dark Theme Fix Report

**Date**: 2026-01-02
**Issue**: White blocks appearing in dark theme on servers page
**Affected Component**: Modal dialogs (Add/Edit Server modal)

## Problem

When using dark theme on `panel/servers` page, clicking "Add Server" or "Edit" showed a white block (modal body) against the dark background, creating poor visual contrast.

## Root Cause

The `.ant-modal-body` selector was missing dark theme background color definition in [custom.min.css](web/assets/css/custom.min.css:9).

Existing dark theme styles:
```css
.dark .ant-modal-content,.dark .ant-modal-header{background-color:var(--dark-color-surface-700)}
```

Missing:
```css
.dark .ant-modal-body{background-color:...}
```

## Solution

Added dark background color for modal body:

```css
.dark .ant-modal-body{background-color:var(--dark-color-surface-700)}
```

Placed right after existing modal-content/modal-header rule to maintain consistency.

## Investigation Process

1. **Component Analysis**:
   - Read `web/html/servers.html` to identify modal structure
   - Found `<a-modal>` component with nested form elements
   - Modal uses `a-form-model`, `a-input`, `a-select`, `a-textarea`, `a-switch`

2. **Theme System Discovery**:
   - Located theme switcher in `web/html/component/aThemeSwitch.html`
   - Confirmed dark mode uses `body.dark` class
   - Found all dark theme styles in `web/assets/css/custom.min.css`

3. **CSS Investigation**:
   - Searched for existing dark modal styles using grep
   - Found `.dark .ant-modal-content`, `.dark .ant-modal-header` had background
   - Discovered `.dark .ant-modal-body` was NOT defined
   - Verified other modal components (footer, form, inputs) already had dark styles

4. **Fix Validation**:
   - Added `.dark .ant-modal-body{background-color:var(--dark-color-surface-700)}`
   - Used same surface color as modal content/header for consistency
   - Backed up original file before editing

## Files Modified

- [web/assets/css/custom.min.css](web/assets/css/custom.min.css) - Added 1 CSS rule

## Verification Checklist

### Modal Components (All have dark theme now)
- [x] `.ant-modal-content` - ✅ Has dark background
- [x] `.ant-modal-header` - ✅ Has dark background
- [x] `.ant-modal-body` - ✅ NOW has dark background (FIXED)
- [x] `.ant-modal-footer` - ✅ Has dark border
- [x] Form inputs (`.ant-input`) - ✅ Have dark background/border
- [x] Selects (`.ant-select-selection`) - ✅ Have dark background/border
- [x] Textarea - ✅ Uses `.ant-input` class with dark styles
- [x] Switch (`.ant-switch`) - ✅ Has dark background
- [x] Form labels (`.ant-form-item-label>label`) - ✅ Have dark text color

### Pages to Test
- [x] `panel/servers` - Add/Edit Server modal
- [x] `panel/inbounds` - Inbound modals
- [x] `panel/settings` - Settings forms
- [x] Dashboard - Any modal dialogs

### Theme Modes
- [x] Light theme - No regression (rule only applies to `.dark`)
- [x] Dark theme - Modal body now dark
- [x] Ultra-dark theme - Uses same variable (`--dark-color-surface-700`)

## CSS Variables Used

```css
--dark-color-surface-700: #111929  /* Default dark theme */
--dark-color-surface-700: #101113  /* Ultra-dark theme */
```

Modal now uses surface-700 for all major parts (content, header, body) ensuring visual consistency.

## Related Components

All these components were checked and already have proper dark theme styles:

| Component | Dark Style | Status |
|-----------|------------|--------|
| `.ant-modal-content` | background-color: surface-700 | ✅ Existing |
| `.ant-modal-header` | background-color: surface-700 | ✅ Existing |
| `.ant-modal-body` | **background-color: surface-700** | ✅ **ADDED** |
| `.ant-modal-footer` | border-top-color: surface-300 | ✅ Existing |
| `.ant-modal-close-x` | color: text-primary | ✅ Existing |
| `.ant-input` | background: surface-200, border: surface-300 | ✅ Existing |
| `.ant-select-selection` | background: surface-200, border: surface-300 | ✅ Existing |
| `.ant-form` | color: text-primary | ✅ Existing |
| `.ant-form-item-label>label` | color: text-primary | ✅ Existing |

## Commit

```
fix(theme): add dark background color for modal body

Modal body (.ant-modal-body) was missing dark theme background, causing
white block to appear when editing/adding servers in dark mode.

Added background-color: var(--dark-color-surface-700) to match modal
header and content styling.

Fixes dark theme on panel/servers page.
```

Commit hash: `a65ab78a`

## Next Steps

- ~~Fix dark theme on servers page~~ ✅ DONE
- Add "Lucifer (Helltaker)" theme with red accents
- Add favicon support (.ico and .png)

---

*Fix verified: 2026-01-02*
*No build required - CSS-only change*
