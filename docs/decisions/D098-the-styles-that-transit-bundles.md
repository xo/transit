# D98. The styles that transit bundles

Status: Decided, amends D65, amended by D100.

D65 says that a command converts the styles of chroma, and that a person
tunes a few styles by hand. It does not say which styles of chroma the
package holds, and it does not add themes that chroma does not have.

At Ken's request, the transit agent asked Gemini and DeepSeek on 2026-10-01
for the most popular color themes of developers, in editors and in
terminals, and compared their lists with the 74 styles of chroma `v2.27.0`.
Most of the popular themes are in chroma, such as Monokai, Solarized,
Dracula, Gruvbox, Nord, One Dark, Tokyo Night, Catppuccin, GitHub, Rosé Pine
and Kanagawa. About 35 popular themes are not.

Ken decided on 2026-10-01, answering question 65:

1. The command converts all 74 styles of chroma `v2.27.0`. usql offers
   every style name of chroma today, so a user of usql keeps the style that
   the user set.
2. The command reads the XML files of chroma from the Go module cache, as
   `test/cmd/regextables` reads the crate of `regex-syntax` from the
   registry of cargo (D60). The version is a constant of the command.
3. transit adds the popular themes that chroma does not have, as styles that
   a person makes by hand (point 2 of "Where the styles come from" in D65).
   The list is VS Code Dark+ and Light+, Material, Palenight, Ayu, Night
   Owl, Everforest, Nightfox, Tomorrow, Atom One Light, Synthwave '84,
   Shades of Purple, Cobalt2, Oceanic Next, Poimandres, Vesper, Sonokai,
   Iceberg, PaperColor, Selenized, Horizon, Moonlight, Winter is Coming,
   Andromeda, Panda, City Lights, Mariana, Alabaster, Melange, Noctis,
   Zenbones, Hybrid, Apprentice, Edge and Spacegray, with their variants.
4. A style takes its colors from the upstream palette of its theme. If the
   theme has a Neovim port, the style takes the colors of the captures from
   that port, so it uses the finer captures. The `source` field names the
   repository, the commit and the license.
5. A theme comes in only if its license lets transit copy its colors under
   the terms of MIT, such as MIT, BSD, ISC or Apache 2.0. Zenburn is under
   GPL 2.0, so it stays out. The package holds the license file of each
   theme that it copies from.
6. The work starts in phase 4, before phase 5, which D65 names.
