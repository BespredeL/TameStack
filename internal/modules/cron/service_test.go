package cron

import (
	"testing"
)

func TestCronScheduleValidation(t *testing.T) {
	validSchedules := [][]string{
		{"*", "*", "*", "*", "*"},
		{"*/5", "*", "*", "*", "*"},
		{"0", "2", "*", "*", "1-5"},
		{"0", "0", "1", "*/2", "*"},
		{"30", "4", "1,15", "*", "5"},
	}

	for _, s := range validSchedules {
		if !isCronSchedule(s) {
			t.Errorf("expected schedule %v to be valid", s)
		}
	}

	invalidSchedules := [][]string{
		{"*", "*", "*"},
		{"bad", "*", "*", "*", "*"},
		{"*", "*", "*", "*", "*", "extra"},
	}

	for _, s := range invalidSchedules {
		if isCronSchedule(s) {
			t.Errorf("expected schedule %v to be invalid", s)
		}
	}
}

func TestMakeJobID(t *testing.T) {
	id1 := makeJobID("* * * * *", "echo 1")
	id2 := makeJobID("* * * * *", "echo 1")
	id3 := makeJobID("0 * * * *", "echo 1")

	if id1 != id2 {
		t.Errorf("expected deterministic hash id")
	}
	if id1 == id3 {
		t.Errorf("expected different hash ids for different expressions")
	}
}
