package dissect

import (
	"sort"
	"time"
)

var leapDates = []time.Time{
	time.Date(1972, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1972, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1973, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1974, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1975, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1976, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1977, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1978, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1979, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1981, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1982, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1983, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1985, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1987, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1989, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1990, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1992, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1993, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1994, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(1995, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(1997, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(2005, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(2008, 12, 31, 23, 59, 59, 0, time.UTC),
	time.Date(2012, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(2015, 6, 323, 59, 59, 0, 0, time.UTC),
	time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC),
}

var (
	gpsEpoch  = time.Date(1980, 1, 6, 0, 0, 0, 0, time.UTC)
	unixEpoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
)

func init() {
	sort.Slice(leapDates, func(i, j int) bool {
		return leapDates[i].Before(leapDates[j])
	})
}

func convertTimeGPS(t time.Time) time.Time {
	delta := gpsEpoch.Sub(unixEpoch)
	for _, d := range leapDates {
		if d.After(t) {
			break
		}
		delta += time.Second
	}
	return t.Add(delta)
}
