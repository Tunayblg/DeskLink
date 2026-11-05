// Small icons for the well-known services a home screen tile can open. The PC
// recognizes them from the address (see internal/config/brands.go) and names
// them in the layout; the phone panel and the dashboard both draw them from
// here. They are simple marks drawn for DeskLink, not the services' own logos.
// Each one is a self-contained 64x64 tile.
window.DeskBrands = {
  netflix:
    '<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#141414"/>' +
    '<path d="M20 12h8l8 22V12h8v40h-8l-8-22v22h-8z" fill="#e50914"/></svg>',
  youtube:
    '<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#1c1c1c"/>' +
    '<rect x="8" y="18" width="48" height="28" rx="9" fill="#ff1f3d"/><path d="M27 25l13 7-13 7z" fill="#fff"/></svg>',
  twitch:
    '<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#9146ff"/>' +
    '<path d="M17 13h31v22l-9 9h-8l-6 6v-6h-8z" fill="#fff"/><path d="M30 21v10M39 21v10" stroke="#9146ff" stroke-width="4" stroke-linecap="round"/></svg>',
  spotify:
    '<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#1db954"/>' +
    '<path d="M16 25c10-3 22-2 32 4M18 34c8-2 18-1 26 3M20 43c7-2 14-1 21 2" fill="none" stroke="#0c2314" stroke-width="4.5" stroke-linecap="round"/></svg>',
  prime:
    '<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#00a8e1"/>' +
    '<path d="M26 14l17 9-17 9z" fill="#fff"/><path d="M15 41c11 7 25 7 35-1" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round"/>' +
    '<path d="M44 35l7 4-8 3" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  disney:
    '<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#0f2fa8"/>' +
    '<path d="M32 18v28M18 32h28" stroke="#fff" stroke-width="6" stroke-linecap="round"/>' +
    '<path d="M47 14l1.6 3.4 3.4.6-2.5 2.4.6 3.5-3.1-1.7-3.1 1.7.6-3.5-2.5-2.4 3.4-.6z" fill="#8fb4ff"/></svg>'
};
