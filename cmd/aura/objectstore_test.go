package main

import (
	"testing"
	"time"
)

func TestParseRestoreArgs(t *testing.T) {
	now := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		args   []string
		asOf   time.Time
		bucket string
		fails  bool
	}{
		{args: nil, asOf: now},
		{args: []string{"--bucket", "aura-assets"}, asOf: now, bucket: "aura-assets"},
		{args: []string{"--as-of", "2026-10-13T23:00:00+02:00"}, asOf: time.Date(2026, 10, 13, 21, 0, 0, 0, time.UTC)},
		{args: []string{"--as-of", "13/10/2026"}, fails: true},
		{args: []string{"--as-of"}, fails: true},
		{args: []string{"aura-assets"}, fails: true},
	}
	for _, c := range cases {
		asOf, bucket, err := parseRestoreArgs(c.args, now)
		if c.fails {
			if err == nil {
				t.Errorf("%v: no error", c.args)
			}
			continue
		}
		if err != nil || !asOf.Equal(c.asOf) || asOf.Location() != time.UTC || bucket != c.bucket {
			t.Errorf("%v = %s, %q, %v; want %s, %q", c.args, asOf, bucket, err, c.asOf, c.bucket)
		}
	}
}
