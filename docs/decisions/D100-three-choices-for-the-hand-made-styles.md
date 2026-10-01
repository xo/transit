# D100. Three choices for the hand-made styles

Status: Decided, amends D98.

The agent that made the styles of D98 raised three points. Ken decided on
2026-10-01, answering question 67:

1. Oceanic Next stays. Its repository holds a copy of the license of Neovim,
   Apache 2.0, with no copyright line of the author of the theme. The
   original scheme says that it is under MIT in its README, and it has no
   license file.
2. Shades of Purple stays. Its MIT license adds the condition that what a
   person builds with it is also under MIT. transit is under MIT.
3. A style that takes its colors from the Vim groups of a theme maps the
   groups to the captures as current Neovim does. Neovim 0.10 and later do
   not link `variable` to the Vim group `Identifier`, so a variable takes the
   color that current Neovim gives it for that theme. This holds for
   PaperColor, Selenized, Panda and Apprentice.

City Lights stays out, because its license is CC BY-NC-ND 4.0.

Ken decided on 2026-10-01 that Mariana comes in, from
`Codextor/npp-mariana-theme`, a theme for Notepad++ under Apache 2.0. Sublime
Text ships the original Mariana under no open license, so the style takes
its colors from that port, from the styles of its lexers. Its license file
has no copyright line of its own.

Ken also asked on 2026-10-01 for a style of Metro Lamps, a VS Code theme
that he made. The style takes its colors from the rules of `tokenColors` in
his `metro-lamps.json`, and it is under the license of transit.
