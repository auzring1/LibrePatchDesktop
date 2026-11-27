package startup

import (
	"testing"
	"time"
)

func TestApproved(t *testing.T) {
	if !isEnabled(nil) || !isEnabled([]byte{2, 0, 0}) || !isEnabled([]byte{6}) || isEnabled([]byte{3}) || isEnabled([]byte{7}) {
		t.Fatal("isEnabled")
	}
	on := approvedValue(true, time.Now())
	off := approvedValue(false, time.Unix(0, 0))
	if len(on) != 12 || !isEnabled(on) || len(off) != 12 || isEnabled(off) || off[11] == 0 && off[10] == 0 {
		t.Fatalf("values %v %v", on, off)
	}
}
