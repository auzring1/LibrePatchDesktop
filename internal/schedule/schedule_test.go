package schedule

import (
	"testing"
	"time"
)

func at(day, hm string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04", "2026-11-"+day+" "+hm, time.UTC)
	return t
}

func TestDue(t *testing.T) {
	s := Schedule{Enabled: true, Weekday: -1, Time: "19:30", RunMissed: false}
	if s.Due(at("26", "19:29"), time.Time{}) {
		t.Fatal("slot from yesterday is too old without RunMissed")
	}
	if !s.Due(at("26", "19:31"), time.Time{}) {
		t.Fatal("just after the time it is due")
	}
	if s.Due(at("26", "19:35"), at("26", "19:31")) {
		t.Fatal("one run per slot")
	}
	if !s.Due(at("27", "19:31"), at("26", "19:31")) {
		t.Fatal("next day is due again")
	}
	if s.Due(at("26", "20:00"), time.Time{}) {
		t.Fatal("30 minutes late is missed when RunMissed is off")
	}
	s.RunMissed = true
	if !s.Due(at("26", "20:00"), time.Time{}) {
		t.Fatal("missed run is made up when RunMissed is on")
	}
	s.Enabled = false
	if s.Due(at("26", "19:31"), time.Time{}) {
		t.Fatal("disabled never runs")
	}
}

func TestWeekday(t *testing.T) {
	// 2026-11-25 is a Wednesday (3).
	s := Schedule{Enabled: true, Weekday: 3, Time: "07:30", RunMissed: true}
	if !s.Due(at("25", "08:00"), time.Time{}) {
		t.Fatal("Wednesday morning")
	}
	if s.Due(at("26", "08:00"), at("25", "07:30")) {
		t.Fatal("Thursday is not due after Wednesday's run")
	}
	if n, ok := s.Next(at("25", "08:00")); !ok || !n.Equal(time.Date(2026, 12, 2, 7, 30, 0, 0, time.UTC)) {
		t.Fatalf("next %v", n)
	}
}

func TestValid(t *testing.T) {
	if (Schedule{Time: "25:00"}).Valid() == nil || (Schedule{Time: "19:30", Weekday: 9}).Valid() == nil || Default().Valid() != nil {
		t.Fatal("validation")
	}
}
