package quarantine

import "time"

type Item struct {
	ID         string
	EnteredAt  time.Time
	NextChecks []time.Time
}

func BuildSchedule(start time.Time, offsets []time.Duration) []time.Time {
	out := make([]time.Time, 0, len(offsets))
	for _, d := range offsets {
		out = append(out, start.Add(d))
	}
	return out
}