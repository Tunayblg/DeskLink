// Line icons for deck buttons (24×24, stroke-based). Shapes adapted from Lucide (ISC license).
// Each entry is [svg inner markup, accent color as "r,g,b"].
window.DESKLINK_ICONS = (function () {
  var speaker = '<path d="M11 5 6 9H2v6h4l5 4V5z"/>';
  var cyan = "56,189,248", violet = "167,139,250", green = "52,211,153",
    amber = "251,191,36", red = "244,63,94", pink = "244,114,182";
  return {
    "music": ['<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>', "30,215,96"],
    "play-pause": ['<path d="M4 5v14l9-7z" fill="currentColor"/><path d="M16.5 5v14M20.5 5v14"/>', cyan],
    "prev": ['<path d="M19 20 9 12l10-8v16z" fill="currentColor"/><path d="M5 19V5"/>', cyan],
    "next": ['<path d="m5 4 10 8-10 8V4z" fill="currentColor"/><path d="M19 5v14"/>', cyan],
    "volume-down": [speaker + '<path d="M15.5 8.5a5 5 0 0 1 0 7"/>', violet],
    "volume-up": [speaker + '<path d="M15.5 8.5a5 5 0 0 1 0 7"/><path d="M19 5a10 10 0 0 1 0 14"/>', violet],
    "mute": [speaker + '<path d="m22 9-6 6M16 9l6 6"/>', violet],
    "screenshot": ['<path d="M3 7V5a2 2 0 0 1 2-2h2M17 3h2a2 2 0 0 1 2 2v2M21 17v2a2 2 0 0 1-2 2h-2M7 21H5a2 2 0 0 1-2-2v-2"/><circle cx="12" cy="12" r="3.5"/>', amber],
    "lock": ['<rect x="4" y="11" width="16" height="10" rx="2.5"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/><path d="M12 15v2"/>', red],
    "gamepad": ['<path d="M6 11h4M8 9v4"/><path d="M15 12h.01M18 10h.01"/><path d="M17.3 5H6.7a4 4 0 0 0-4 3.6l-.9 8.1A2.5 2.5 0 0 0 4.3 19.5c.9 0 1.7-.5 2.1-1.3L8 15h8l1.6 3.2c.4.8 1.2 1.3 2.1 1.3a2.5 2.5 0 0 0 2.5-2.8l-.9-8.1A4 4 0 0 0 17.3 5z"/>', pink],
    "folder": ['<path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.7-.9l-.8-1.2A2 2 0 0 0 7.9 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2z"/>', amber],
    "monitor": ['<rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/>', cyan],
    "mic": ['<rect x="9" y="2" width="6" height="12" rx="3"/><path d="M19 10v1a7 7 0 0 1-14 0v-1M12 18v4"/>', green],
    "mic-off": ['<rect x="9" y="2" width="6" height="12" rx="3"/><path d="M19 10v1a7 7 0 0 1-14 0v-1M12 18v4"/><path d="m3 3 18 18"/>', red],
    "globe": ['<circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20"/>', cyan],
    "terminal": ['<path d="m4 17 6-6-6-6M12 19h8"/>', green],
    "moon": ['<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z"/>', "129,140,248"],
    "power": ['<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.8 0"/>', red]
  };
})();
