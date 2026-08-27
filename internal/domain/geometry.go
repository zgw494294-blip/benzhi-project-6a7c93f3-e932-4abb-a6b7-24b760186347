package domain

import "math"

func ZonesOverlap(a, b TrialZone) bool {
	for i := range a.BoundaryPoints {
		a1 := a.BoundaryPoints[i]
		a2 := a.BoundaryPoints[(i+1)%len(a.BoundaryPoints)]
		for j := range b.BoundaryPoints {
			b1 := b.BoundaryPoints[j]
			b2 := b.BoundaryPoints[(j+1)%len(b.BoundaryPoints)]
			if segmentsIntersect(a1, a2, b1, b2) {
				return true
			}
		}
	}
	return pointInside(a.BoundaryPoints[0], b.BoundaryPoints) || pointInside(b.BoundaryPoints[0], a.BoundaryPoints)
}

func segmentsIntersect(a, b, c, d Point) bool {
	o1 := orientation(a, b, c)
	o2 := orientation(a, b, d)
	o3 := orientation(c, d, a)
	o4 := orientation(c, d, b)
	if o1*o2 < 0 && o3*o4 < 0 {
		return true
	}
	return (nearZero(o1) && onSegment(a, c, b)) || (nearZero(o2) && onSegment(a, d, b)) ||
		(nearZero(o3) && onSegment(c, a, d)) || (nearZero(o4) && onSegment(c, b, d))
}

func orientation(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func nearZero(value float64) bool { return math.Abs(value) < 1e-9 }

func onSegment(a, p, b Point) bool {
	return p.X >= math.Min(a.X, b.X)-1e-9 && p.X <= math.Max(a.X, b.X)+1e-9 &&
		p.Y >= math.Min(a.Y, b.Y)-1e-9 && p.Y <= math.Max(a.Y, b.Y)+1e-9
}

func pointInside(point Point, polygon []Point) bool {
	inside := false
	for i, current := range polygon {
		previous := polygon[(i+len(polygon)-1)%len(polygon)]
		crosses := (current.Y > point.Y) != (previous.Y > point.Y)
		if crosses && point.X < (previous.X-current.X)*(point.Y-current.Y)/(previous.Y-current.Y)+current.X {
			inside = !inside
		}
	}
	return inside
}
