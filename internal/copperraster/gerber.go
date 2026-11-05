package copperraster

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Film is what drawing a Gerber film reported about it.
type Film struct {
	// Etch is the copper layer the film plots, from Allegro's "G04 Layer: ETCH/<name>" header line,
	// or "" when the film names none.
	Etch string
	// Regions counts the filled regions drawn, and Clear the draws, flashes and regions made with
	// clear polarity. A clear one erases the copper under it, as Allegro cuts a pour's voids
	// (agni 1357).
	Regions, Clear int
	// Draws and Flashes count what was drawn.
	Draws, Flashes int
}

type aperture struct {
	shape    byte         // 'C', 'R', 'O', 'M' for the centre-line macro, or 'P' for the outline macro
	poly     [][2]float64 // the outline macro's vertices, nanometres from the flash point
	w, h     float64
	mx, my   float64 // the macro's centre offset
	rotation float64 // the macro's rotation, degrees
}

// DrawGerber draws the dark tracks (draws and arcs) of an RS-274X film into tracks and its dark
// flashes into lands. The two may be the same bitmap. It reads the subset
// Allegro 17.2 writes: absolute leading-zero-suppressed coordinates in inches or millimetres, circle,
// rectangle and obround apertures, the centre-line macro primitive (21), linear and multi-quadrant
// circular interpolation, regions, and polarity changes. Anything outside it (step and repeat, other
// macro primitives, a mirrored, offset, scaled or negative image) is an error naming the statement,
// so a film the subset cannot draw is never drawn wrong.
func DrawGerber(r io.Reader, tracks, lands *Bitmap) (Film, error) {
	g := &gerber{tracks: tracks, lands: lands, apertures: map[int]aperture{}, macros: map[string]aperture{}, dark: true, interp: 1, unit: 25.4e6}
	if err := g.run(bufio.NewReader(r)); err != nil {
		return g.film, err
	}
	return g.film, nil
}

type gerber struct {
	tracks      *Bitmap
	lands       *Bitmap
	film        Film
	apertures   map[int]aperture
	macros      map[string]aperture
	cur         int
	x, y        float64 // current point, nanometres
	intDigits   int
	decDigits   int
	unit        float64 // nanometres per file unit
	interp      int     // 1 linear, 2 clockwise, 3 counter-clockwise
	multiQuad   bool
	region      bool
	contour     [][2]float64   // the region contour being drawn
	contours    [][][2]float64 // the region's finished contours
	dark        bool
	fsSeen      bool
	sawEnd      bool
	pendingAMID string
}

func (g *gerber) run(r *bufio.Reader) error {
	for {
		c, err := r.ReadByte()
		if err == io.EOF {
			if !g.sawEnd {
				return fmt.Errorf("gerber: no M02 end of file")
			}
			return nil
		}
		if err != nil {
			return err
		}
		switch {
		case c == '%':
			block, err := r.ReadString('%')
			if err != nil {
				return fmt.Errorf("gerber: unterminated extended command")
			}
			for _, cmd := range strings.Split(strings.TrimSuffix(block, "%"), "*") {
				if cmd = strings.TrimSpace(cmd); cmd != "" {
					if err := g.extended(cmd); err != nil {
						return err
					}
				}
			}
		case c == '\n' || c == '\r' || c == ' ':
		default:
			rest, err := r.ReadString('*')
			if err != nil {
				return fmt.Errorf("gerber: unterminated word %q", string(c)+rest)
			}
			if err := g.word(string(c) + strings.TrimSuffix(rest, "*")); err != nil {
				return err
			}
		}
	}
}

func (g *gerber) extended(cmd string) error {
	if g.pendingAMID != "" {
		return g.macroBody(cmd)
	}
	switch {
	case strings.HasPrefix(cmd, "FS"):
		var xi, xd, yi, yd int
		if _, err := fmt.Sscanf(cmd, "FSLAX%1d%1dY%1d%1d", &xi, &xd, &yi, &yd); err != nil || xi != yi || xd != yd {
			return fmt.Errorf("gerber: unsupported format %q (only leading-zero-suppressed absolute)", cmd)
		}
		g.intDigits, g.decDigits, g.fsSeen = xi, xd, true
	case cmd == "MOIN":
		g.unit = 25.4e6
	case cmd == "MOMM":
		g.unit = 1e6
	case cmd == "IPPOS", cmd == "IR0", cmd == "LPD":
		g.dark = true
	case cmd == "LPC":
		g.dark = false
	case strings.HasPrefix(cmd, "OF"), strings.HasPrefix(cmd, "MI"), strings.HasPrefix(cmd, "SF"):
		if !identityImage(cmd) {
			return fmt.Errorf("gerber: unsupported image transform %q", cmd)
		}
	case strings.HasPrefix(cmd, "AM"):
		g.pendingAMID = strings.TrimPrefix(cmd, "AM")
	case strings.HasPrefix(cmd, "AD"):
		return g.aperture(cmd)
	default:
		return fmt.Errorf("gerber: unsupported extended command %q", cmd)
	}
	return nil
}

// identityImage accepts an image offset, mirror or scale statement only at its identity value.
func identityImage(cmd string) bool {
	switch cmd {
	case "OFA0B0", "OFA0.00000B0.00000", "MIA0B0", "SFA1B1", "SFA1.00000B1.00000":
		return true
	}
	return false
}

// macroBody reads the one primitive an aperture macro may hold: a centre-line rectangle (21), or an
// outline polygon (4), which Allegro writes for a custom pad.
func (g *gerber) macroBody(cmd string) error {
	id := g.pendingAMID
	g.pendingAMID = ""
	cmd = strings.Join(strings.Fields(cmd), "")
	f := strings.Split(cmd, ",")
	num := func(s string) (float64, error) {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("gerber: macro %s: %v", id, err)
		}
		return n, nil
	}
	switch {
	case len(f) == 7 && f[0] == "21" && f[1] == "1":
		v := make([]float64, 5)
		for i := range v {
			n, err := num(f[i+2])
			if err != nil {
				return err
			}
			v[i] = n
		}
		g.macros[id] = aperture{shape: 'M', w: v[0] * g.unit, h: v[1] * g.unit, mx: v[2] * g.unit, my: v[3] * g.unit, rotation: v[4]}
		return nil
	case len(f) >= 4 && f[0] == "4" && f[1] == "1":
		n, err := strconv.Atoi(f[2])
		if err != nil || len(f) != 3+2*(n+1)+1 {
			return fmt.Errorf("gerber: macro %s outline %q does not hold %s vertices and a rotation", id, cmd, f[2])
		}
		rot, err := num(f[len(f)-1])
		if err != nil {
			return err
		}
		sn, cs := math.Sincos(rot * math.Pi / 180)
		a := aperture{shape: 'P'}
		for i := 0; i <= n; i++ {
			x, err := num(f[3+2*i])
			if err != nil {
				return err
			}
			y, err := num(f[4+2*i])
			if err != nil {
				return err
			}
			a.poly = append(a.poly, [2]float64{(x*cs - y*sn) * g.unit, (x*sn + y*cs) * g.unit})
		}
		g.macros[id] = a
		return nil
	}
	return fmt.Errorf("gerber: unsupported macro %s primitive %q (only a centre-line rectangle, 21, or an outline, 4)", id, cmd)
}

func (g *gerber) aperture(cmd string) error {
	body := strings.TrimPrefix(cmd, "ADD")
	i := 0
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		i++
	}
	code, err := strconv.Atoi(body[:i])
	if err != nil {
		return fmt.Errorf("gerber: aperture %q: %v", cmd, err)
	}
	tmpl, params, _ := strings.Cut(body[i:], ",")
	if m, ok := g.macros[tmpl]; ok {
		g.apertures[code] = m
		return nil
	}
	var dims []float64
	for _, p := range strings.Split(params, "X") {
		n, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return fmt.Errorf("gerber: aperture %q: %v", cmd, err)
		}
		dims = append(dims, n*g.unit)
	}
	switch {
	case tmpl == "C" && len(dims) == 1:
		g.apertures[code] = aperture{shape: 'C', w: dims[0], h: dims[0]}
	case (tmpl == "R" || tmpl == "O") && len(dims) == 2:
		g.apertures[code] = aperture{shape: tmpl[0], w: dims[0], h: dims[1]}
	default:
		return fmt.Errorf("gerber: unsupported aperture %q", cmd)
	}
	return nil
}

func (g *gerber) word(w string) error {
	switch {
	case strings.HasPrefix(w, "G04"):
		if name, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimPrefix(w, "G04")), "Layer:"); ok {
			if etch, ok := strings.CutPrefix(strings.TrimSpace(name), "ETCH/"); ok {
				g.film.Etch = etch
			}
		}
		return nil
	case w == "M02":
		g.sawEnd = true
		return nil
	case w == "G36":
		g.region = true
		g.contour, g.contours = nil, nil
		return nil
	case w == "G37":
		g.region = false
		g.fillRegion()
		return nil
	case w == "G75":
		g.multiQuad = true
		return nil
	case w == "G74":
		return fmt.Errorf("gerber: single-quadrant arcs (G74) are not read")
	case strings.HasPrefix(w, "G54D"):
		n, err := strconv.Atoi(w[4:])
		if err != nil {
			return fmt.Errorf("gerber: %q: %v", w, err)
		}
		g.cur = n
		return nil
	}
	if strings.HasPrefix(w, "D") && !strings.HasPrefix(w, "D0") {
		n, err := strconv.Atoi(w[1:])
		if err != nil {
			return fmt.Errorf("gerber: %q: %v", w, err)
		}
		g.cur = n
		return nil
	}
	return g.operation(w)
}

// operation reads a coordinate word such as "G02X123Y456I7J8D01".
func (g *gerber) operation(w string) error {
	if !g.fsSeen {
		return fmt.Errorf("gerber: coordinates before a format statement")
	}
	vals := map[byte]string{}
	for i := 0; i < len(w); {
		key := w[i]
		j := i + 1
		for j < len(w) && (w[j] == '-' || w[j] == '+' || (w[j] >= '0' && w[j] <= '9')) {
			j++
		}
		if j == i+1 || !strings.ContainsRune("GXYIJD", rune(key)) {
			return fmt.Errorf("gerber: unsupported word %q", w)
		}
		vals[key] = w[i+1 : j]
		i = j
	}
	if gv, ok := vals['G']; ok {
		switch gv {
		case "01", "1":
			g.interp = 1
		case "02", "2":
			g.interp = 2
		case "03", "3":
			g.interp = 3
		default:
			return fmt.Errorf("gerber: unsupported G code in %q", w)
		}
	}
	x, y := g.x, g.y
	if v, ok := vals['X']; ok {
		x = g.coord(v)
	}
	if v, ok := vals['Y']; ok {
		y = g.coord(v)
	}
	i, j := g.coord(vals['I']), g.coord(vals['J'])
	d, hasD := vals['D']
	if !hasD {
		if _, ok := vals['X']; !ok {
			if _, ok := vals['Y']; !ok {
				return nil // a bare G code
			}
		}
		d = "01" // a modal draw
	}
	switch d {
	case "02", "2":
		if g.region {
			g.closeContour()
		}
	case "01", "1":
		g.draw(x, y, i, j)
	case "03", "3":
		g.flash(x, y)
	default:
		return fmt.Errorf("gerber: unsupported D code in %q", w)
	}
	g.x, g.y = x, y
	return nil
}

func (g *gerber) coord(s string) float64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return float64(n) / math.Pow10(g.decDigits) * g.unit
}

func (g *gerber) draw(x, y, i, j float64) {
	if g.region {
		if len(g.contour) == 0 {
			g.contour = append(g.contour, [2]float64{g.x, g.y})
		}
		g.contour = append(g.contour, g.arcPoints(x, y, i, j)...)
		return
	}
	defer g.polarity()()
	a := g.apertures[g.cur]
	width := math.Min(a.w, a.h)
	g.film.Draws++
	if g.interp == 1 {
		g.stroke(g.x, g.y, x, y, width)
		return
	}
	cx, cy := g.x+i, g.y+j
	r := math.Hypot(g.x-cx, g.y-cy)
	start := math.Atan2(g.y-cy, g.x-cx)
	end := math.Atan2(y-cy, x-cx)
	sweep := end - start
	if g.interp == 3 && sweep <= 0 {
		sweep += 2 * math.Pi
	}
	if g.interp == 2 && sweep >= 0 {
		sweep -= 2 * math.Pi
	}
	n := max(1, int(math.Ceil(math.Abs(sweep)/0.1)))
	px, py := g.x, g.y
	for k := 1; k <= n; k++ {
		ang := start + sweep*float64(k)/float64(n)
		qx, qy := cx+r*math.Cos(ang), cy+r*math.Sin(ang)
		if k == n {
			qx, qy = x, y
		}
		g.stroke(px, py, qx, qy, width)
		px, py = qx, qy
	}
}

func (g *gerber) flash(x, y float64) {
	if g.region {
		return
	}
	defer g.polarity()()
	a := g.apertures[g.cur]
	g.film.Flashes++
	flashInto(g.lands, a, x, y)
	if !g.dark && g.tracks != g.lands {
		flashInto(g.tracks, a, x, y)
	}
}

// stroke draws a dark line into tracks, and a clear one out of both bitmaps.
func (g *gerber) stroke(ax, ay, bx, by, width float64) {
	g.tracks.Line(ax, ay, bx, by, width)
	if !g.dark && g.tracks != g.lands {
		g.lands.Line(ax, ay, bx, by, width)
	}
}

func flashInto(b *Bitmap, a aperture, x, y float64) {
	switch a.shape {
	case 'C':
		b.Disc(x, y, a.w)
	case 'R':
		b.Rect(x, y, a.w, a.h, 0)
	case 'O':
		b.Oval(x, y, a.w, a.h, 0)
	case 'M':
		s, c := math.Sincos(a.rotation * math.Pi / 180)
		b.Rect(x+a.mx*c-a.my*s, y+a.mx*s+a.my*c, a.w, a.h, a.rotation)
	case 'P':
		b.Polygon(x, y, a.poly)
	}
}

// arcPoints is the points a draw from the current point to (x, y) passes through after the start:
// the end alone for a line, and a point every tenth of a radian for an arc.
func (g *gerber) arcPoints(x, y, i, j float64) [][2]float64 {
	if g.interp == 1 {
		return [][2]float64{{x, y}}
	}
	cx, cy := g.x+i, g.y+j
	r := math.Hypot(g.x-cx, g.y-cy)
	start := math.Atan2(g.y-cy, g.x-cx)
	sweep := math.Atan2(y-cy, x-cx) - start
	if g.interp == 3 && sweep <= 0 {
		sweep += 2 * math.Pi
	}
	if g.interp == 2 && sweep >= 0 {
		sweep -= 2 * math.Pi
	}
	n := max(1, int(math.Ceil(math.Abs(sweep)/0.1)))
	pts := make([][2]float64, 0, n)
	for k := 1; k < n; k++ {
		ang := start + sweep*float64(k)/float64(n)
		pts = append(pts, [2]float64{cx + r*math.Cos(ang), cy + r*math.Sin(ang)})
	}
	return append(pts, [2]float64{x, y})
}

func (g *gerber) closeContour() {
	if len(g.contour) >= 3 {
		g.contours = append(g.contours, g.contour)
	}
	g.contour = nil
}

// fillRegion fills each contour of the region just ended. A dark region is copper, drawn into
// lands. A clear region erases whatever was drawn under it, in both bitmaps, which is how a film
// cuts a pour's voids.
func (g *gerber) fillRegion() {
	g.closeContour()
	for _, c := range g.contours {
		g.film.Regions++
		if !g.dark {
			g.film.Clear++
		}
		g.lands.Rings([][][2]float64{c}, g.dark)
		if !g.dark && g.tracks != g.lands {
			g.tracks.Rings([][][2]float64{c}, false)
		}
	}
	g.contours = nil
}

// polarity sets both bitmaps erasing while the polarity is clear, and returns the call that sets
// them back. A clear draw or flash cuts the copper under it in either bitmap, since a void cuts a
// track as surely as a pad.
func (g *gerber) polarity() func() {
	if g.dark {
		return func() {}
	}
	g.film.Clear++
	g.tracks.erasing, g.lands.erasing = true, true
	return func() { g.tracks.erasing, g.lands.erasing = false, false }
}
