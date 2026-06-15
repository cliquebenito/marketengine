package domain

import "time"

func UTCDay(t time.Time) time.Time {
	return t.UTC().Truncate(24 * time.Hour)
}

func MustParseDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic("MustParseDay: " + err.Error())
	}
	return t.UTC()
}
