# Artwork

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

## The emblem is drawn, not generated

A badge emblem was generated and rejected: it came back with three discs, not
four, scattered rather than in a line. A four-in-a-row logo showing three discs
contradicts the name of the game, and the mark is four circles and a rounded
badge — geometry with one correct answer. It is drawn in `pkg/render`
(`drawEmblem`) instead, which is also crisp at any size and needs no asset.

Generated art earns its place in atmosphere, where there is no correct answer
and a model's guess is as good as a hand's. It does not earn its place in a
mark that has to mean something exact.
