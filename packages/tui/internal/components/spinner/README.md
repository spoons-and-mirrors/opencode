# OpenCode Spinner Component

This component creates an animated "O" and "C" logo spinner for the OpenCode TUI application. The animation shows the letters "O" and "C" with a trail effect, where pixels light up and fade as they move around each letter's perimeter.

## How It Works

### Animation Phases

The spinner operates in 4 distinct phases that cycle continuously:

1. **PhaseOAnimating**: The "O" letter animates with a moving light trail
2. **PhaseOSleeping**: The "O" fades out while waiting
3. **PhaseCAnimating**: The "C" letter animates with a moving light trail
4. **PhaseCSleeping**: The "C" fades out while waiting

### Letter Structure

Both letters are **4 pixels tall** and **2 pixels wide** (vertical rectangles):

#### O Letter (10 positions)

```
██ ██  (positions 0, 9)
██ ██  (positions 8, 6)
██ ██  (positions 1, 5)
██ ██  (positions 2, 4)
```

- Animates **counter-clockwise** starting from top-left
- Position sequence: 0 → 8 → 1 → 2 → 4 → 5 → 6 → 9 → (repeat)

#### C Letter (8 positions)

```
██ ██  (positions 0, 4)
██     (position 6)
██     (position 7)
██ ██  (positions 3, 5)
```

- Animates **clockwise** starting from top-left
- Position sequence: 0 → 6 → 7 → 3 → 5 → 4 → (repeat)
- Right side is open (forming the "C" shape)

### Fade Trail System

Each position has a **fade level** from 0-4:

- **4**: Brightest (current position) - Primary color
- **3**: Fade level 1 - Primary color with faint
- **2**: Fade level 2 - Muted color
- **1**: Fade level 3 - Muted color with faint
- **0**: Invisible - Background color

## Configuration Parameters

### Timing Controls (in `New()` function)

```go
interval:        120 * time.Millisecond,  // Speed of each animation frame
oLoopFrames:     10,                      // Frames for O to complete one loop
cLoopFrames:     8,                       // Frames for C to complete one loop
sleepFrames:     8,                       // Frames to sleep between animations
```

**Key Tweaks:**

- **`interval`**: Lower = faster animation (try 80ms for faster, 200ms for slower)
- **`sleepFrames`**: Lower = less pause between letters (try 2-4 for overlap)
- **`oLoopFrames`/`cLoopFrames`**: Should match number of positions in each letter

### Animation Speed Examples

```go
// Super fast animation
interval: 60 * time.Millisecond,
sleepFrames: 2,

// Slow, dramatic animation
interval: 200 * time.Millisecond,
sleepFrames: 15,

// Overlapping animation (C starts before O finishes)
sleepFrames: 3,
```

## How to Modify

### 1. Change Animation Speed

Edit the `interval` in the `New()` function:

```go
interval: 80 * time.Millisecond,  // Faster
interval: 200 * time.Millisecond, // Slower
```

### 2. Adjust Sleep Between Letters

Edit `sleepFrames` in the `New()` function:

```go
sleepFrames: 2,  // Short pause (creates overlap)
sleepFrames: 15, // Long pause (dramatic effect)
```

### 3. Change Letter Colors

Modify the fade levels in `ViewO()` and `ViewC()`:

```go
case 4: // Current position
    style = lipgloss.NewStyle().Foreground(t.Secondary()) // Use different color
case 3: // Fade levels...
```

### 4. Reverse Animation Direction

In the `Update()` method:

**For O (make clockwise):**

```go
s.currentOFrame = (s.currentOFrame + 1) % len(s.oFadeFrames)
```

**For C (make counter-clockwise):**

```go
s.currentCFrame = (s.currentCFrame - 1 + len(s.cFadeFrames)) % len(s.cFadeFrames)
```

### 5. Change Letter Shapes

Modify the position arrays in `ViewO()` and `ViewC()`:

```go
// Example: Make O wider (3x4 instead of 2x4)
oPositions := [][]int{
    {0, 9, 10}, // top row - now 3 pixels wide
    {8, -1, 6}, // second row - empty center
    {1, -1, 5}, // third row - empty center
    {2, 4, 11}, // bottom row - now 3 pixels wide
}
```

### 6. Add More Fade Levels

Increase fade trail length by:

1. Changing the initial fade value: `s.oFadeFrames[s.currentOFrame] = 6` (instead of 4)
2. Adding more cases in the style switch

### 7. Create Different Animation Patterns

Replace the phase logic in `Update()` with custom patterns:

```go
// Example: Both letters animate simultaneously
case PhaseOAnimating:
    // Animate both O and C at the same time
    // ... O animation code ...
    // ... C animation code ...
```

## File Structure

- **`spinner.go`**: Main component file
- **`packages/tui/internal/tui/tui.go`**: Integration with main TUI
- **Usage**: Add `s.spinner.ViewOCOnly()` to display just the animated letters

## Integration

The spinner is integrated into the main TUI home screen and can be displayed using:

- `s.spinner.View()`: Full view with spacing
- `s.spinner.ViewOCOnly()`: Just the O and C letters
- `s.spinner.ViewO()`: Only the O letter
- `s.spinner.ViewC()`: Only the C letter
