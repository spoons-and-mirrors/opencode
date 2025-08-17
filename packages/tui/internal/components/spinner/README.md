# OpenCode 3D ASCII "O" Spinner

This component creates a rotating 3D ASCII "O" spinner inspired by the famous donut.c algorithm. The animation shows a three-dimensional letter "O" rotating on multiple axes with dynamic ASCII-based lighting and z-buffering.

## How It Works

### 3D Math Foundation

The spinner uses the same mathematical principles as the classic "donut.c" program:

1. **3D Geometry**: Creates a 3D "O" shape using cylindrical coordinates
2. **3D Rotations**: Applies rotation matrices for X, Y, and Z axes
3. **3D→2D Projection**: Projects 3D coordinates to 2D screen space
4. **Z-Buffering**: Maintains depth information to render correctly
5. **Dynamic Lighting**: Calculates surface normals and lighting

### Technical Implementation

#### 3D "O" Generation

```go
// Create the "O" as points between inner and outer radius
outerRadius := 3.0
innerRadius := 1.5
thickness := 0.8

for theta := 0.0; theta < 2*math.Pi; theta += 0.15 {
    for depth := -thickness; depth <= thickness; depth += 0.3 {
        for r := innerRadius; r <= outerRadius; r += 0.3 {
            x := r * math.Cos(theta)
            y := r * math.Sin(theta)
            z := depth
            // ... process point
        }
    }
}
```

#### 3D Rotation Matrices

The spinner applies three rotation matrices in sequence:

- **X-axis rotation**: `rotX += 0.05`
- **Y-axis rotation**: `rotY += 0.03`
- **Z-axis rotation**: `rotZ += 0.02`

#### 3D→2D Projection

```go
func (s *OpenCodeSpinner) project3D(p Point3D) (int, int, float64) {
    K2 := 8.0  // Distance from viewer
    K1 := 40.0 // Scale factor

    z := p.Z + K2
    x := int(float64(s.width)/2 + K1*p.X/z)
    y := int(float64(s.height)/2 - K1*p.Y/z)

    return x, y, 1.0/z // return 1/z for z-buffering
}
```

#### ASCII Lighting System

Uses character palette based on lighting intensity:

```go
chars := ".,-~:;=!*#$@"  // dimmest to brightest
```

Each pixel's character is determined by:

1. Calculate surface normal
2. Compute dot product with light direction `(0, 0.7, -0.7)`
3. Map lighting value to character index
4. Apply z-buffer test

### Animation Parameters

```go
// Rotation speeds (radians per frame)
s.rotX += 0.05  // X-axis rotation
s.rotY += 0.03  // Y-axis rotation
s.rotZ += 0.02  // Z-axis rotation

// Timing
interval: 80 * time.Millisecond  // Frame rate

// Dimensions
width, height := 24, 12  // ASCII canvas size

// 3D Geometry
outerRadius := 3.0   // Outer edge of "O"
innerRadius := 1.5   // Inner hole of "O"
thickness := 0.8     // Depth of letter

// Projection
K1 := 40.0  // Scale factor
K2 := 8.0   // Distance from viewer
```

### Lighting & Styling

The spinner uses the current theme for styling:

- **Bright characters** (`@`, `$`, `#`): Primary color, bold
- **Medium characters** (`*`, `!`, `=`): Primary color
- **Dim characters** (`;`, `:`, `~`): Muted color
- **Faint characters** (`-`, `,`, `.`): Muted color, faint

Light direction is from above and behind the viewer: `(0, 0.7, -0.7)`

## Customization

### Speed Control

```go
// Faster rotation
s.rotX += 0.08
s.rotY += 0.05
s.rotZ += 0.03

// Slower rotation
s.rotX += 0.02
s.rotY += 0.015
s.rotZ += 0.01

// Faster framerate
interval: 50 * time.Millisecond
```

### Size & Resolution

```go
// Larger display
width, height := 40, 20

// Higher detail
theta += 0.1  // smaller angle steps
r += 0.2      // more radius steps
```

### Geometry Changes

```go
// Thicker "O"
thickness := 1.2

// Wider "O"
outerRadius := 4.0
innerRadius := 1.0

// Different shape ratios
// Make it more elliptical by scaling Y
y := r * math.Sin(theta) * 0.7  // vertically compressed
```

### Lighting Direction

```go
// Light from the right
lightDir := Point3D{X: 1.0, Y: 0, Z: 0}

// Light from below
lightDir := Point3D{X: 0, Y: -1.0, Z: 0}

// Multiple light sources (average the results)
```

### Character Sets

```go
// High contrast
chars := " .:!*#@"

// Unicode blocks
chars := " ░▒▓█"

// Custom artistic
chars := " .oO0"
```

## Technical Notes

### Z-Buffer Algorithm

Each pixel maintains depth information (`1/z`) to ensure correct rendering:

```go
if ooz > s.zbuffer[idx] {
    s.zbuffer[idx] = ooz
    s.output[idx] = chars[lightingIndex]
}
```

### Surface Normal Calculation

Normals are approximated for lighting:

```go
normal := Point3D{
    X: math.Cos(theta),
    Y: math.Sin(theta),
    Z: 0,  // simplified for cylindrical surface
}
rotatedNormal := s.rotate3D(normal)
lighting := calculateLighting(rotatedNormal)
```

### Performance Optimizations

- Pre-calculate sin/cos values when possible
- Use integer arithmetic for screen coordinates
- Limit point generation resolution based on final display size
- Clear buffers efficiently each frame

## Integration

The 3D spinner integrates with the OpenCode TUI:

- `View()`: Renders the full 3D "O" animation
- `ViewO()`: Same as View() for compatibility
- `ViewC()`: Simple static "C" for compatibility
- `ViewOCOnly()`: Just the animated "O"

The spinner automatically handles:

- Theme integration via lipgloss styling
- Bubbletea message passing for animation updates
- Graceful start/stop of animation loops

## Inspiration

This implementation is inspired by the classic [donut.c](https://www.a1k0n.net/2011/07/20/donut-math.html) by Andy Sloan, adapted for:

- ASCII letter rendering instead of torus geometry
- OpenCode branding with the letter "O"
- Terminal/TUI integration with Go and Bubbletea
- Theme-aware styling and modern terminal features

The mathematical foundation remains the same: 3D rotation matrices, perspective projection, z-buffering, and ASCII-based lighting.
