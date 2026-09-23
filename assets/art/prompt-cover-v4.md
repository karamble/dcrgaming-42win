# FOUR2WIN diagonal cover (v4)

Generated with the built-in imagegen tool, guided by the imagegen skill.
Selected output: `cover-v4.png`. The existing logo is unchanged and composited separately.

## Board position

Turquoise moves first; one-based columns:

`6, 6, 7, 1, 2, 4, 5, 7, 2, 6, 4, 3, 6, 6, 7, 1, 3, 2, 4, 5, 2, 5, 5`

Top row first; T = turquoise, B = blue, dot = empty:

```text
. . . . . . .
. . . . . B .
. T . . T T .
. B . T B B T
B T T T B B B
B T B B T T T
```

Exactly one winning line: turquoise at (column, row from bottom)
(2,1), (3,2), (4,3), (5,4). No earlier win in the move sequence.
The selected image was visually checked against this layout after the correction pass.

## Initial edit

Input: `cover-v2.png`. The first result misplaced several discs and bent the
highlight; it was not selected.

```text
Use case: precise-object-edit
Asset type: cinematic FOUR2WIN game loading-screen background.
Input image 1: edit target, the arena cover. Preserve its composition, arena, foreground flying discs, camera angle, metal board frame, dark title space, palette and lighting.
Replace ONLY the seated discs within the central board with a believable late-game position: varied gravity-supported mixed turquoise and cobalt stacks, and ONE winning turquoise diagonal rising from lower left to upper right. No horizontal win, no blue win.
The board is exactly 7 columns by 6 rows. Follow this exact verified legal 23-move position. T means turquoise disc, B means cobalt blue disc, dot means EMPTY hollow dark hole. Grid below goes TOP row to BOTTOM row, each line LEFT to RIGHT:
. . . . . . .
. . . . . B .
. T . . T T .
. B . T B B T
B T T T B B B
B T B B T T T
The ONLY highlighted winning connection is four TURQUOISE discs at: column 2 bottom row; column 3 second row from bottom; column 4 third row from bottom; column 5 fourth row from bottom. A thin straight turquoise beam connects these four centers diagonally in the board's perspective. Make ONLY these four discs brightly luminous; all other pieces use ordinary turquoise or cobalt material so the winning diagonal is obvious.
Gravity is essential: pieces sit on other pieces, with empty spaces only above stacks. Preserve exactly the specified empty top row and varied column heights. No extra discs, no arbitrary recoloring, no second four-in-a-row. The played position should look competitive, not a staged easy win.
Keep all exterior flying foreground discs and arena unchanged. No text, logos, symbols or watermark; the existing approved logo is composited separately by the game.
```

## Correction pass — selected

Input: the first generated result above.

```text
Use case: precise-object-edit.
Edit target: the provided arena image. Preserve the entire scene and board geometry. Correct six specific board cells and the mistaken bent winning beam. Do NOT change the bottom two rows, which are already correct.
Coordinate convention: columns 1–7 run left to right. Rows 1–6 run TOP to BOTTOM, following the board's perspective slant.
Required cell changes:
- Row 2 column 6: insert a cobalt BLUE disc (above the current blue disc).
- Row 3 column 5: insert a TURQUOISE disc in the currently empty hole.
- Row 3 column 6: change the current blue disc to TURQUOISE.
- Row 4 column 4: insert a TURQUOISE disc in the currently empty hole.
- Row 4 column 5: change the currently glowing turquoise disc to ordinary cobalt BLUE.
- Row 4 column 6: change the turquoise disc to ordinary cobalt BLUE.
Keep row 3 column 2 turquoise, row 4 column 2 blue, row 4 column 7 turquoise.
Erase the old crooked beam and recreate a single STRAIGHT diagonal through precisely these four turquoise cells: (row6,column2), (row5,column3), (row4,column4), (row3,column5). These four centers form a straight line in perspective. Illuminate ONLY those four turquoise discs strongly. In particular row5 column4 must be a normal non-glowing turquoise disc, NOT part of the win.
Final grid, rows TOP to BOTTOM:
. . . . . . .
. . . . . B .
. T . . T T .
. B . T B B T
B T T T B B B
B T B B T T T
T=turquoise, B=blue, dot=empty dark hollow hole. No floating discs, gaps under discs, bent winning lines, extra wins or changes to exterior scenery. No text.
```
