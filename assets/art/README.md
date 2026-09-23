# Artwork

## FOUR2WIN branding (v1)

Generated with the built-in imagegen tool, using the imagegen skill. Exact
generation and compact-variant edit prompts are in
[`prompts-four2win-v1.md`](prompts-four2win-v1.md).

- `four2win-logo-v1.png`: illustrated transparent logo with a decorative board,
  four connected turquoise discs, and loose playing pieces. Cover and lobby.
- `four2win-wordmark-v1.png`: matching transparent lettering without the board
  or loose pieces, for table, seating, help, results and receipt headers.

Both original generated PNGs are embedded unmodified. At decode time, layout
ignores fully transparent margins, preserving the alpha and aspect ratio.
Missing artwork falls back to the text FOUR2WIN. This is a visual rebrand only:
bridge game IDs, signatures, executable names and profiles are unchanged.

## Diagonal-win loading cover (v4)

`cover-v4.png` replaces the v2 cover's impossible board position and the v3
candidate's unconvincing bottom-row finish. Generated with the built-in imagegen
tool using the imagegen skill; prompts and the verified position are in
[`prompt-cover-v4.md`](prompt-cover-v4.md). The FOUR2WIN logos are unchanged.

The arena board has seven columns and six rows, with mixed gravity-supported
stacks after 23 alternating moves. Exactly one turquoise diagonal wins, from
column 2 at the bottom to column 5 on the fourth row from the bottom. The blue
player has no winning line. Previous covers are retained as unused assets.

## Desktop atmosphere (v2)

Generated with the built-in imagegen tool. Exact prompts are in
[`prompts-v2.md`](prompts-v2.md). Selected PNGs are embedded and preserved
unmodified; no runtime downloads are needed.

- `cover-v2.png`: previous arena cover, superseded by v4 after review of its
  board position. Retained for provenance; no longer embedded.
- `lobby-v2.png`: a quieter tabletop scene with negative space for the interface.
- `tabletop-v2.png`: subdued material underneath the playable board.

The code draws the loading state, playable 7×6 board and discs. Generated
boards in the artwork and logo are decorative, not playable positions.
Missing art falls back to the branded navy canvas. Sound is synthesized locally
using Ebitengine's Oto backend, isolated so device errors disable only sound.

## Original art (v1, retained)

Generated with braibot (`flux/schnell`) on 2026-09-18 and post-processed here.
Prompts are kept beside the images so a regeneration starts from what was asked
for rather than from memory.

- `backdrop-b.jpg` — the source plate. Prompt: `prompt-backdrop-v1.txt`.
- `lobby-backdrop.png` — what the client embeds: centre-cropped to 3:2, scaled
  to 960×640, then mixed 38% over the lobby's own navy. The mix is the point.
  At full strength the glow competes with the headline; at 38% it reads as
  artwork and white text over it stays white text.
- `backdrop-a.jpg` — a second plate from the same prompt, four discs in a row
  and centred. Unused: the composition puts its subject exactly where the
  headline goes. Kept as a candidate for a cover or splash screen.

The original code-drawn four-disc badge has been superseded by the illustrated
FOUR2WIN logo at the user's request. Gameplay geometry remains code-drawn.
