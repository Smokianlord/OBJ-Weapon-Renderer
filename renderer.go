package main

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"
)

type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3    { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3    { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Mul(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vec3) Len() float64 { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l < 1e-20 {
		return Vec3{0, 0, 1}
	}
	return a.Mul(1 / l)
}

type FaceVertex struct{ V, N int }
type Tri struct{ A, B, C FaceVertex }
type Mesh struct {
	V []Vec3
	N []Vec3
	T []Tri
}

type RenderOptions struct {
	Width                 int
	Height                int
	Margin                float64 // 0..0.4
	View                  string  // auto, x, y, z
	Mirror                bool
	FlipVertical          bool
	UseOriginalNormals    bool
	AA                    int // 1 or 2
	BackgroundTransparent bool
	BaseGray              uint8
	Contrast              float64
}

func DefaultRenderOptions() RenderOptions {
	return RenderOptions{Width: 4000, Height: 2000, Margin: 0.055, View: "auto", Mirror: false, FlipVertical: false, UseOriginalNormals: true, AA: 2, BackgroundTransparent: true, BaseGray: 108, Contrast: 1.0}
}

func parseIndex(s string, n int) (int, error) {
	if s == "" {
		return -1, nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1, err
	}
	if i > 0 {
		i--
	} else if i < 0 {
		i = n + i
	} else {
		return -1, errors.New("OBJ indices are 1-based")
	}
	if i < 0 || i >= n {
		return -1, fmt.Errorf("OBJ index %d out of range %d", i, n)
	}
	return i, nil
}

func LoadOBJ(path string) (*Mesh, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m := &Mesh{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024), 16*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fs := strings.Fields(line)
		if len(fs) == 0 {
			continue
		}
		switch fs[0] {
		case "v":
			if len(fs) < 4 {
				continue
			}
			x, e1 := strconv.ParseFloat(fs[1], 64)
			y, e2 := strconv.ParseFloat(fs[2], 64)
			z, e3 := strconv.ParseFloat(fs[3], 64)
			if e1 != nil || e2 != nil || e3 != nil {
				return nil, fmt.Errorf("bad vertex at line %d", lineNo)
			}
			m.V = append(m.V, Vec3{x, y, z})
		case "vn":
			if len(fs) < 4 {
				continue
			}
			x, e1 := strconv.ParseFloat(fs[1], 64)
			y, e2 := strconv.ParseFloat(fs[2], 64)
			z, e3 := strconv.ParseFloat(fs[3], 64)
			if e1 != nil || e2 != nil || e3 != nil {
				return nil, fmt.Errorf("bad normal at line %d", lineNo)
			}
			m.N = append(m.N, Vec3{x, y, z}.Norm())
		case "f":
			if len(fs) < 4 {
				continue
			}
			verts := make([]FaceVertex, 0, len(fs)-1)
			for _, tok := range fs[1:] {
				p := strings.Split(tok, "/")
				vi, er := parseIndex(p[0], len(m.V))
				if er != nil {
					return nil, fmt.Errorf("line %d: %w", lineNo, er)
				}
				ni := -1
				if len(p) >= 3 && p[2] != "" {
					ni, er = parseIndex(p[2], len(m.N))
					if er != nil {
						return nil, fmt.Errorf("line %d: %w", lineNo, er)
					}
				}
				verts = append(verts, FaceVertex{vi, ni})
			}
			for i := 1; i+1 < len(verts); i++ {
				m.T = append(m.T, Tri{verts[0], verts[i], verts[i+1]})
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(m.V) == 0 || len(m.T) == 0 {
		return nil, errors.New("OBJ contains no renderable triangles")
	}
	return m, nil
}

type camBasis struct {
	right, up, forward Vec3
	center             Vec3
	scale              float64
}

func bbox(m *Mesh) (Vec3, Vec3) {
	mn, mx := m.V[0], m.V[0]
	for _, p := range m.V[1:] {
		if p.X < mn.X {
			mn.X = p.X
		}
		if p.Y < mn.Y {
			mn.Y = p.Y
		}
		if p.Z < mn.Z {
			mn.Z = p.Z
		}
		if p.X > mx.X {
			mx.X = p.X
		}
		if p.Y > mx.Y {
			mx.Y = p.Y
		}
		if p.Z > mx.Z {
			mx.Z = p.Z
		}
	}
	return mn, mx
}

func axisVec(i int) Vec3 {
	if i == 0 {
		return Vec3{1, 0, 0}
	}
	if i == 1 {
		return Vec3{0, 1, 0}
	}
	return Vec3{0, 0, 1}
}

func makeCamera(m *Mesh, opt RenderOptions) camBasis {
	mn, mx := bbox(m)
	center := mn.Add(mx).Mul(0.5)
	ext := []float64{mx.X - mn.X, mx.Y - mn.Y, mx.Z - mn.Z}
	depthAxis := 0
	switch strings.ToLower(opt.View) {
	case "x":
		depthAxis = 0
	case "y":
		depthAxis = 1
	case "z":
		depthAxis = 2
	default:
		depthAxis = 0
		if ext[1] < ext[depthAxis] {
			depthAxis = 1
		}
		if ext[2] < ext[depthAxis] {
			depthAxis = 2
		}
	}
	remain := []int{}
	for i := 0; i < 3; i++ {
		if i != depthAxis {
			remain = append(remain, i)
		}
	}
	horizAxis, vertAxis := remain[0], remain[1]
	if ext[vertAxis] > ext[horizAxis] {
		horizAxis, vertAxis = vertAxis, horizAxis
	}
	// The largest remaining extent is horizontal; the other is vertical.
	// This keeps long weapon models horizontal while still working for conventional Z-up assets.
	right := axisVec(horizAxis)
	up := axisVec(vertAxis)
	forward := right.Cross(up).Norm()
	// Ensure forward aligns with selected depth axis, not its negative arbitrarily.
	target := axisVec(depthAxis)
	if math.Abs(forward.Dot(target)) < 0.5 {
		// If basis selection became degenerate, rebuild explicitly.
		forward = target
		if depthAxis == 0 {
			right = Vec3{0, 1, 0}
			up = Vec3{0, 0, 1}
		}
		if depthAxis == 1 {
			right = Vec3{1, 0, 0}
			up = Vec3{0, 0, 1}
		}
		if depthAxis == 2 {
			right = Vec3{1, 0, 0}
			up = Vec3{0, 1, 0}
		}
	} else if forward.Dot(target) < 0 {
		forward = forward.Mul(-1)
	}
	if opt.Mirror {
		right = right.Mul(-1)
	}
	if opt.FlipVertical {
		up = up.Mul(-1)
	}

	minx, miny := math.Inf(1), math.Inf(1)
	maxx, maxy := math.Inf(-1), math.Inf(-1)
	for _, p := range m.V {
		q := p.Sub(center)
		x := q.Dot(right)
		y := q.Dot(up)
		if x < minx {
			minx = x
		}
		if x > maxx {
			maxx = x
		}
		if y < miny {
			miny = y
		}
		if y > maxy {
			maxy = y
		}
	}
	spanx := maxx - minx
	spany := maxy - miny
	usableW := float64(opt.Width) * (1 - 2*opt.Margin)
	usableH := float64(opt.Height) * (1 - 2*opt.Margin)
	scale := math.Min(usableW/spanx, usableH/spany)
	// Recenter projected bbox precisely.
	projCenter := right.Mul((minx + maxx) / 2).Add(up.Mul((miny + maxy) / 2))
	center = center.Add(projCenter)
	return camBasis{right, up, forward, center, scale}
}

type pv struct {
	x, y, z float64
	n       Vec3
}

func edge(ax, ay, bx, by, cx, cy float64) float64 { return (cx-ax)*(by-ay) - (cy-ay)*(bx-ax) }

func RenderOBJ(inPath, outPath string, opt RenderOptions) error {
	if opt.Width < 64 || opt.Height < 64 {
		return errors.New("resolution too small")
	}
	if opt.Width > 12000 || opt.Height > 12000 {
		return errors.New("resolution too large")
	}
	if opt.AA != 2 {
		opt.AA = 1
	}
	if opt.Margin < 0 {
		opt.Margin = 0
	}
	if opt.Margin > 0.35 {
		opt.Margin = 0.35
	}
	m, err := LoadOBJ(inPath)
	if err != nil {
		return err
	}
	cam := makeCamera(m, opt)
	ss := opt.AA
	W, H := opt.Width*ss, opt.Height*ss
	scl := cam.scale * float64(ss)
	cx, cy := float64(W)/2, float64(H)/2
	pix := make([]uint8, W*H*4)
	depth := make([]float64, W*H)
	for i := range depth {
		depth[i] = math.Inf(-1)
	}

	key := Vec3{-0.35, 0.55, 0.76}.Norm()
	fill := Vec3{0.6, -0.25, 0.55}.Norm()
	rim := Vec3{-0.7, -0.35, 0.5}.Norm()
	view := Vec3{0, 0, 1}
	base := float64(opt.BaseGray)
	contrast := opt.Contrast
	if contrast <= 0 {
		contrast = 1
	}

	conv := func(fv FaceVertex) pv {
		p := m.V[fv.V].Sub(cam.center)
		n := Vec3{}
		if opt.UseOriginalNormals && fv.N >= 0 && fv.N < len(m.N) {
			n = m.N[fv.N]
		}
		return pv{x: cx + p.Dot(cam.right)*scl, y: cy - p.Dot(cam.up)*scl, z: p.Dot(cam.forward), n: Vec3{n.Dot(cam.right), n.Dot(cam.up), n.Dot(cam.forward)}.Norm()}
	}

	for _, t := range m.T {
		a, b, c := conv(t.A), conv(t.B), conv(t.C)
		if !opt.UseOriginalNormals || t.A.N < 0 || t.B.N < 0 || t.C.N < 0 {
			pa := m.V[t.A.V]
			pb := m.V[t.B.V]
			pc := m.V[t.C.V]
			fn := pb.Sub(pa).Cross(pc.Sub(pa)).Norm()
			cn := Vec3{fn.Dot(cam.right), fn.Dot(cam.up), fn.Dot(cam.forward)}.Norm()
			a.n, b.n, c.n = cn, cn, cn
		}
		area := edge(a.x, a.y, b.x, b.y, c.x, c.y)
		if math.Abs(area) < 1e-9 {
			continue
		}
		minx := int(math.Floor(math.Min(a.x, math.Min(b.x, c.x))))
		maxx := int(math.Ceil(math.Max(a.x, math.Max(b.x, c.x))))
		miny := int(math.Floor(math.Min(a.y, math.Min(b.y, c.y))))
		maxy := int(math.Ceil(math.Max(a.y, math.Max(b.y, c.y))))
		if minx < 0 {
			minx = 0
		}
		if miny < 0 {
			miny = 0
		}
		if maxx >= W {
			maxx = W - 1
		}
		if maxy >= H {
			maxy = H - 1
		}
		inv := 1.0 / area
		for y := miny; y <= maxy; y++ {
			py := float64(y) + 0.5
			for x := minx; x <= maxx; x++ {
				px := float64(x) + 0.5
				w0 := edge(b.x, b.y, c.x, c.y, px, py) * inv
				w1 := edge(c.x, c.y, a.x, a.y, px, py) * inv
				w2 := 1 - w0 - w1
				const eps = -1e-8
				if w0 < eps || w1 < eps || w2 < eps {
					continue
				}
				z := w0*a.z + w1*b.z + w2*c.z
				idx := y*W + x
				if z <= depth[idx] {
					continue
				}
				depth[idx] = z
				n := a.n.Mul(w0).Add(b.n.Mul(w1)).Add(c.n.Mul(w2)).Norm()
				// Flip back-facing normals for two-sided neutral display; avoids black faces without changing geometry.
				if n.Z < 0 {
					n = n.Mul(-1)
				}
				d1 := math.Max(0, n.Dot(key))
				d2 := math.Max(0, n.Dot(fill))
				d3 := math.Max(0, n.Dot(rim))
				diffuse := 0.34 + 0.42*d1 + 0.15*d2 + 0.09*d3
				// Small deterministic specular, capped to avoid white fireflies.
				h := key.Add(view).Norm()
				spec := math.Pow(math.Max(0, n.Dot(h)), 48) * 0.12
				lum := diffuse + spec
				lum = (lum-0.5)*contrast + 0.5
				if lum < 0.18 {
					lum = 0.18
				}
				if lum > 0.92 {
					lum = 0.92
				}
				v := uint8(math.Max(0, math.Min(255, base*lum/0.58)))
				o := idx * 4
				pix[o] = v
				pix[o+1] = uint8(math.Min(255, float64(v)+2))
				pix[o+2] = uint8(math.Min(255, float64(v)+4))
				pix[o+3] = 255
			}
		}
	}

	out := image.NewNRGBA(image.Rect(0, 0, opt.Width, opt.Height))
	if ss == 1 {
		for y := 0; y < opt.Height; y++ {
			for x := 0; x < opt.Width; x++ {
				i := (y*W + x) * 4
				if pix[i+3] == 0 {
					out.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 0})
				} else {
					out.SetNRGBA(x, y, color.NRGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]})
				}
			}
		}
	} else {
		// Alpha-aware downsample so transparent edges do not get white/dark RGB contamination.
		for y := 0; y < opt.Height; y++ {
			for x := 0; x < opt.Width; x++ {
				var sr, sg, sb, sa int
				for yy := 0; yy < ss; yy++ {
					for xx := 0; xx < ss; xx++ {
						i := (((y*ss + yy) * W) + (x*ss + xx)) * 4
						a := int(pix[i+3])
						sa += a
						sr += int(pix[i]) * a
						sg += int(pix[i+1]) * a
						sb += int(pix[i+2]) * a
					}
				}
				if sa == 0 {
					out.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 0})
					continue
				}
				samples := ss * ss
				aa := uint8((sa + samples/2) / samples)
				rr := uint8(sr / sa)
				gg := uint8(sg / sa)
				bb := uint8(sb / sa)
				out.SetNRGBA(x, y, color.NRGBA{rr, gg, bb, aa})
			}
		}
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	return enc.Encode(f, out)
}
