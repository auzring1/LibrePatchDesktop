// Package schedule decides when the automatic update run is due.
package schedule

import (
	"fmt"
	"time"
)

// Schedule is the user's choice on the Scheduler page.
type Schedule struct {
	Enabled bool `json:"enabled"`
	// Weekday is -1 for every day, otherwise 0 (Sunday) to 6 (Saturday).
	Weekday int `json:"weekday"`
	// Time of day as "15:04" (24 hours, local time).
	Time string `json:"time"`
	// RunMissed runs the update when the scheduled time passed while the app was not running.
	RunMissed bool `json:"runMissed"`
}

// Default is shown the first time: every day at 19:30, missed runs allowed.
func Default() Schedule { return Schedule{Weekday: -1, Time: "19:30", RunMissed: true} }

// Valid reports whether the time and weekday make sense.
func (s Schedule) Valid() error {
	if _, err := time.Parse("15:04", s.Time); err != nil {
		return fmt.Errorf("time must look like 19:30")
	}
	if s.Weekday < -1 || s.Weekday > 6 {
		return fmt.Errorf("weekday must be -1 (every day) or 0-6")
	}
	return nil
}

// graceWindow is how late a run may start when RunMissed is off (the app checks every 30 seconds).
const graceWindow = 10 * time.Minute

// Last returns the most recent scheduled moment at or before now, within the last week.
func (s Schedule) Last(now time.Time) (time.Time, bool) {
	hm, err := time.Parse("15:04", s.Time)
	if err != nil {
		return time.Time{}, false
	}
	for d := 0; d <= 7; d++ {
		day := now.AddDate(0, 0, -d)
		t := time.Date(day.Year(), day.Month(), day.Day(), hm.Hour(), hm.Minute(), 0, 0, now.Location())
		if t.After(now) {
			continue
		}
		if s.Weekday == -1 || int(t.Weekday()) == s.Weekday {
			return t, true
		}
	}
	return time.Time{}, false
}

// Due reports whether an update run should start now. lastRun is when the previous run started (zero if never).
// A scheduled moment is used once: after a run has started for it, it is not due again.
func (s Schedule) Due(now, lastRun time.Time) bool {
	if !s.Enabled {
		return false
	}
	t, ok := s.Last(now)
	if !ok || !t.After(lastRun) {
		return false
	}
	return s.RunMissed || now.Sub(t) <= graceWindow
}

// Next returns the next scheduled moment after now.
func (s Schedule) Next(now time.Time) (time.Time, bool) {
	hm, err := time.Parse("15:04", s.Time)
	if err != nil {
		return time.Time{}, false
	}
	for d := 0; d <= 8; d++ {
		day := now.AddDate(0, 0, d)
		t := time.Date(day.Year(), day.Month(), day.Day(), hm.Hour(), hm.Minute(), 0, 0, now.Location())
		if t.After(now) && (s.Weekday == -1 || int(t.Weekday()) == s.Weekday) {
			return t, true
		}
	}
	return time.Time{}, false
}
