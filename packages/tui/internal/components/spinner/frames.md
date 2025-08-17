# Frame-based Spinner Animation

This file contains 5 frames for the OpenCode spinner animation.
Each frame uses:

- `.` for empty space (renders as space)
- `x` for filled pixels (renders as ▀ character)

## Frame 1

```
xx..
.xx.
..xx
...x
```

## Frame 2

```
.xx.
..xx
...x
....
```

## Frame 3

```
..xx
...x
....
x...
```

## Frame 4

```
...x
....
x...
xx..
```

## Frame 5

```
....
x...
xx..
.xx.
```

## Usage

The spinner cycles through these 5 frames at 10fps (100ms intervals).
You can customize the frames by editing the `frames` array in the `New()` function in spinner.go.

## Customization

- Change the pattern by modifying the frame strings
- Add more frames for smoother animation
- Use different characters by modifying the `renderFrame()` function
- Adjust speed by changing the `interval` value (currently 100ms for 10fps)


--- I MEANT THIS.. SORRY.. IT WAS NOT SAVED....

........
..xxxx..
..x..x..
..x..x..
..xxxx..
........
---
........
........
..xxxx..
..x..x..
..x..x..
..xxxx..
---
........
........
........
..xxxx..
..x..x..
.xxxxxx.
---
........
..xxxx..
..x..x..
..x..x..
..xxxx..
........
---
..xxxx..
..x..x..
..x..x..
..xxxx..
........
........