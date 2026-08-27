# Fluentra brand assets

The `Logo/` folder this replaces held generator output and its leftovers. This
one holds what the product actually uses, plus the originals it was derived
from, so `Logo/` can be deleted.

## Colours

| Token | Light | Dark | Where |
|---|---|---|---|
| Brand | `#011AD1` | `#7194F8` | the mark only |
| Primary | `#2563EB` | `#60A5FA` | every action and navigation state |
| Success | `#22C55E` | `#34D399` | progress, completion |

**The brand blue is not the primary blue.** `#011AD1` is deeper and more violet
than the `#2563EB` the interface acts in. Both appear in the sidebar, a few
pixels apart. That is a live question, not an oversight — decide it rather than
letting it settle by accident. The dark value `#7194F8` is taken from the
original artwork's own palette, because `#011AD1` reads at 1.4:1 on the dark
surface and vanishes.

## Files

| File | Use |
|---|---|
| `fluentra-mark.svg` | the mark, redrawn as two stroked paths, `currentColor`, 362 bytes |
| `source/` | the original artwork, untouched |

The app does not load `fluentra-mark.svg` over the network. It is inlined as
`web/src/components/layout/BrandMark.tsx` so it can inherit `--color-brand`.
Keep the two in step; the component is the copy that ships.

The favicon set and the PWA icons live in `web/public/`, because Vite serves
that directory at the site root and `index.html` references them by absolute
path.

## Why the mark was redrawn

`source/fluentra-mark.svg` is a raster trace, not vector artwork: 119 paths, of
which 26 are `fill="white"` — they paint a background and the counters of the F
rather than leaving them open. So the file cannot sit on any surface that is
not white, and it costs 81 kB to say so. Both source SVGs carry the same paths
and differ only in `viewBox`.

The replacement was measured off the 1024px raster rather than eyeballed:

```
stroke width   117
stem centre    x = 175      bottom cap centre y = 870
arm centre     y = 141      right cap centre  x = 759
corner radius  190          (solved from the outer edge at x=180, y=166)
inner bar      y = 415, left cap centre x = 445
tail centre    x = 451      bottom cap centre y = 868
```

Fidelity to the original silhouette is **0.944 intersection-over-union**, from
rasterising both at 256 px and comparing ink masks. The remaining difference is
the trace's hand-drawn irregularity, which a geometric mark is meant to lose.

## There is no wordmark SVG

`source/fluentra-logo-*.png` sets "Fluentra" in a typeface that is not in this
repository, so it cannot be reproduced as vector without the font. The app
composes the mark with live text instead, which translates, scales and stays
selectable. Use the PNG only where live text is impossible — an email
signature, an OG image.
