# Favicon Setup

This directory contains favicon files for the 3X-UI panel.

## Required Files

Place your favicon files in this directory (`web/assets/`) with the following names:

| Filename | Size | Format | Purpose |
|----------|------|--------|---------|
| `favicon.ico` | 16x16, 32x32, 48x48 (multi-size) | ICO | Legacy browser support |
| `favicon-16x16.png` | 16x16 | PNG | Modern browsers (small) |
| `favicon-32x32.png` | 32x32 | PNG | Modern browsers (medium) |
| `apple-touch-icon.png` | 180x180 | PNG | iOS home screen icon |

## File Locations

```
web/assets/
├── favicon.ico              # Main favicon (multi-size ICO)
├── favicon-16x16.png        # 16x16 PNG
├── favicon-32x32.png        # 32x32 PNG
└── apple-touch-icon.png     # 180x180 PNG for iOS
```

## How to Generate Favicons

### Option 1: Online Generator (Easiest)

1. Go to [favicon.io](https://favicon.io/) or [realfavicongenerator.net](https://realfavicongenerator.net/)
2. Upload your logo/image (recommended size: 512x512 or larger)
3. Download the generated favicon package
4. Extract and place files in `web/assets/` directory

### Option 2: Using ImageMagick (Command Line)

```bash
# Start with a high-resolution source image (512x512 or larger)
SOURCE="logo.png"

# Generate PNG sizes
convert $SOURCE -resize 16x16 web/assets/favicon-16x16.png
convert $SOURCE -resize 32x32 web/assets/favicon-32x32.png
convert $SOURCE -resize 180x180 web/assets/apple-touch-icon.png

# Generate ICO (multi-size)
convert $SOURCE -resize 16x16 favicon-16.png
convert $SOURCE -resize 32x32 favicon-32.png
convert $SOURCE -resize 48x48 favicon-48.png
convert favicon-16.png favicon-32.png favicon-48.png web/assets/favicon.ico
rm favicon-16.png favicon-32.png favicon-48.png
```

### Option 3: Using GIMP (GUI)

1. Open your logo in GIMP
2. For each size:
   - Image → Scale Image → Set width/height
   - File → Export As → Save as PNG
3. For `.ico` file:
   - Install "ICO" export plugin
   - Scale to 48x48
   - File → Export As → Save as `.ico`

## Design Recommendations

- **Simple design**: Favicons are tiny, keep them simple
- **High contrast**: Ensure visibility on both light and dark backgrounds
- **Square aspect ratio**: Use square logos (1:1 ratio)
- **No text**: Avoid small text, use symbols/icons instead
- **Transparent background**: Use transparent PNG for better flexibility
- **Test both themes**: Check how it looks in light and dark modes

## Current Default

If no favicon files are present, browsers will use their default icon (blank page or generic globe).

## HTML Implementation

Favicons are automatically loaded via `web/html/common/page.html`:

```html
<link rel="icon" type="image/x-icon" href="{{ .base_path }}assets/favicon.ico">
<link rel="icon" type="image/png" sizes="16x16" href="{{ .base_path }}assets/favicon-16x16.png">
<link rel="icon" type="image/png" sizes="32x32" href="{{ .base_path }}assets/favicon-32x32.png">
<link rel="apple-touch-icon" sizes="180x180" href="{{ .base_path }}assets/apple-touch-icon.png">
```

No additional configuration needed - just place the files in the correct location.

## Troubleshooting

### Favicon not showing?

1. **Clear browser cache**: Hard refresh with `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac)
2. **Check file permissions**: Ensure files are readable
   ```bash
   chmod 644 web/assets/favicon*.png web/assets/apple-touch-icon.png
   chmod 644 web/assets/favicon.ico
   ```
3. **Verify file paths**: Files must be in `web/assets/` directory
4. **Check browser console**: Look for 404 errors

### Different favicon on different pages?

The panel uses a single set of favicons across all pages. If you see different icons, clear your browser cache.

### iOS not showing icon?

Ensure `apple-touch-icon.png` is exactly 180x180 pixels and in PNG format.

---

*Last updated: 2026-01-02*
