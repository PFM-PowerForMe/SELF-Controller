package controller

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	cases := []struct {
		name string
		now  time.Time
		at   string
		want string
	}{
		{"同一天稍晚", time.Date(2026, 10, 7, 20, 0, 0, 0, loc), "21:01", "2026-10-07 21:01:00"},
		{"正好到点顺延一天", time.Date(2026, 10, 7, 21, 1, 0, 0, loc), "21:01", "2026-10-08 21:01:00"},
		{"已经过了顺延一天", time.Date(2026, 10, 7, 23, 59, 0, 0, loc), "21:01", "2026-10-08 21:01:00"},
		{"跨月", time.Date(2026, 10, 31, 22, 0, 0, 0, loc), "21:01", "2026-11-01 21:01:00"},
		{"跨年零点", time.Date(2026, 12, 31, 0, 0, 0, 0, loc), "00:00", "2027-01-01 00:00:00"},
	}
	for _, c := range cases {
		got, err := nextRun(c.now, c.at, loc)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if formatted := got.Format("2006-01-02 15:04:05"); formatted != c.want {
			t.Errorf("%s: 期望 %s 得到 %s", c.name, c.want, formatted)
		}
	}
}

func TestNextRunInvalid(t *testing.T) {
	for _, at := range []string{"", "21", "25:00", "21:61", "ab:cd", "21:01:00"} {
		if _, err := nextRun(time.Now(), at, time.UTC); err == nil {
			t.Errorf("%q 应该报错", at)
		}
	}
}
