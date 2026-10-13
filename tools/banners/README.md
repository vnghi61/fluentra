# Dashboard slides

The three promotional slides `cmd/seed` puts in the dashboard carousel
(`core.banners`, kind `slide`). Design: option 6, "Tối giản hiện đại".

| File | Rendered to | Links to |
|---|---|---|
| `slides.html` `#slide-1` | `db/fixtures/banners/slide-1.jpg` | `/learn` |
| `slides.html` `#slide-2` | `db/fixtures/banners/slide-2.jpg` | `/practice/my-words` |
| `slides.html` `#slide-3` | `db/fixtures/banners/slide-3.jpg` | `/exams` |

## Changing a slide

1. Edit `slides.html`. It uses the app's own font (`web/public/fonts`) and the
   tokens from `web/src/index.css`; keep to them.
2. `node tools/banners/render.mjs` (needs `pnpm -C web install` for Playwright).
3. Look at the three JPEGs, then commit them with the HTML.

The next seed in each environment uploads the new pictures. It does not touch
a slide's title, order or on/off switch: those belong to the admin screen.

## The frame

Each slide is 1500×500 (3:1), the frame phones show. The desktop carousel is
4:1 and crops 62.5 px off the top and the bottom, so everything that matters
stays between y = 72 and y = 428. On a phone the slide is about 343 px wide:
the headline stays readable, the small print is decoration.

## Illustrations

`art/*.png` are [Microsoft Fluent Emoji](https://github.com/microsoft/fluentui-emoji)
3D images (potted plant, books, beach with umbrella, bullseye, light bulb),
copyright Microsoft Corporation, used under the MIT License in `art/LICENSE`.
