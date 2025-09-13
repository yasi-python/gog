package stats

import "math"

// WilsonLowerBound returns the lower bound of the Wilson score interval for a proportion p = successes/n.
// If you want failure rate LB, pass failures as successes over n.
func WilsonLowerBound(successes, n int, z float64) float64 {
	if n == 0 {
		return 0.0
	}
	p := float64(successes) / float64(n)
	den := 1.0 + (z*z)/float64(n)
	center := p + (z*z)/(2.0*float64(n))
	rad := z * math.Sqrt((p*(1.0-p)+ (z*z)/(4.0*float64(n)))/float64(n))
	return (center - rad) / den
}