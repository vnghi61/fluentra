// Renders tools/banners/slides.html into the three dashboard slides that
// cmd/seed loads: db/fixtures/banners/slide-<n>.jpg, 1500×500.
//
//   node tools/banners/render.mjs            (after `pnpm -C web install`)
//
// Edit slides.html, render, look at the JPEGs, commit both. The seed uploads
// whatever is in db/fixtures/banners, so a re-render reaches every
// environment on its next seed.
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
// Playwright comes from the web app's install; nothing new is added.
const require = createRequire(path.join(root, "web/package.json"));
const { chromium } = require("@playwright/test");

const out = path.join(root, "db/fixtures/banners");
const browser = await chromium.launch();
try {
  const page = await browser.newPage({
    viewport: { width: 1600, height: 1700 },
    deviceScaleFactor: 1,
  });
  await page.goto(pathToFileURL(path.join(here, "slides.html")).href);
  await page.evaluate(() => document.fonts.ready);
  await page.waitForLoadState("networkidle");
  for (const n of [1, 2, 3]) {
    // JPEG at 1500×500: the shape the admin form asks for, inside the
    // server's 1600×900 bound, and what ck_banners_image_key expects.
    await page.locator(`#slide-${n}`).screenshot({
      path: path.join(out, `slide-${n}.jpg`),
      type: "jpeg",
      quality: 88,
    });
  }
} finally {
  await browser.close();
}
console.log(`rendered 3 slides into ${path.relative(root, out)}`);
