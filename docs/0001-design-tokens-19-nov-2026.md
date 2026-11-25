# 0001 - Figma Design Tokens (19 Nov 2026)

Status: Active
Source: Figma `MedVault - Redesign Dashboard UI` (`Wt4wwa2QJOIPVWbhesTjgX`, ver `2423131685551667227`).

## Purpose & Placement

Consolidates Figma UI values into CSS custom properties to prevent guesswork in HTML.
- `web/assets/tokens.css`: Custom properties with `--mv-ui-` prefix.
- `web/layout.html`: Imports `tokens.css` and registers `--color-clinical-*` in Tailwind `@theme`.
- Static asset: covered by `web/embed.go` with cache buster `?v={{.AssetVersion}}`. Zero CDN.

## Token Reference

| Group | Primary Values |
| :--- | :--- |
| Canvas & Surface | `#f5faf8`, `#ffffff`, `#f7fbfa`, `#f1f8f6` |
| Gradients (Sidebar/Ledger) | `#04302c` 0%, `#064e46` 50%, `#0b6b60` 100% (`.mv-ui-gradient-panel`) |
| Ink / Text | `#102a26`, `#5b716d`, `#9fc4bd`, `#7a908c`, `#d5f0eb` |
| Accent | `#0f9f91`, `#087f75`, `#075e57`, `#2dd4bf`, `#99f6e4`, `#5eead4` |
| Borders | `#dbe7e3`, `#b7ddd6` |
| Success / Danger | Success `#166534`, `#ecfdf3`, `#bbf7d0`, `#86efac` / Danger `#9f1239`, `#e11d48`, `#fff1f2`, `#fecdd3` |
| Radii | 8, 10, 12, 14, 16, 18, 20, 24, 28, 999 px |
| Typography | Inter with system fallbacks, weights 400, 500, 600, 700; sizes 11-34 px |
| 1440 Grid | Sidebar 264 px, inset 24 px, main offset 312 px, main 1104 px, gap 24 px, card gap 16 px |
| Motion | Standard `cubic-bezier(0.4, 0, 0.2, 1)`, enter `(0, 0, 0.2, 1)`, exit `(0.4, 0, 1, 1)`, stagger 60ms, total 700ms |

## Rules

1. Use `--mv-ui-` prefix to avoid colliding with auth tokens in `layout.html`.
2. Inter falls back to system font stack (no external CDN font downloads).
3. Set motion duration to `0ms` when `prefers-reduced-motion: reduce`.
4. Run `go test ./web/...` after changes to verify asset hash updates.
