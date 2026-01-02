# Report

## Theme switch bug

### Stuck elements
- Servers table endpoint text and last-seen subtext
- Empty placeholders in Version/Tags columns
- Add/Edit modal hint text
- Servers stats value colors

### Cause
- Inline `style` and `:value-style` hardcoded colors on `web/html/servers.html`
- Lucifer theme overrides referenced dark tokens for layout/placeholder surfaces
- Lucifer hover/interactive accents used dark theme green values

### Fix
- Replaced inline styles with servers-page classes and theme tokens in `web/html/servers.html`
- Added servers page CSS variables for muted text + stats colors
- Corrected Lucifer surfaces and accent colors to use Lucifer tokens

### How to verify
1. Open `/panel/servers`.
2. Switch themes: light -> dark -> lucifer -> light.
3. Confirm endpoint text, last-seen, placeholders, stats, and modal hints match the active theme.
4. Repeat the switch 3-5 times and confirm no colors remain from the previous theme.
