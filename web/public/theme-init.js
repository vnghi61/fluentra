// Applies the theme before React mounts, so a learner never sees one painted
// frame in the wrong colours. Kept as a separate blocking script (not inline)
// so the CSP can run `script-src 'self'` with no `'unsafe-inline'`.
(function () {
  try {
    var stored = localStorage.getItem("fluentra.theme");
    var dark =
      stored === "dark" ||
      (stored !== "light" &&
        window.matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.classList.toggle("dark", dark);
  } catch (e) {
    /* private mode: fall back to the light default */
  }
})();
