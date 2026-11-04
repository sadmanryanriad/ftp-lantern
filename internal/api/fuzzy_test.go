package api

import "testing"

func TestSlidingWindowMatch(t *testing.T) {
	if !SlidingWindowMatch("visoin", "visionquest", 2) {
		t.Errorf("expected visoin to match visionquest within distance 2")
	}

	if !SlidingWindowMatch("quest", "visionquest", 0) {
		t.Errorf("expected exact substring quest to match visionquest")
	}

	if SlidingWindowMatch("completelydifferent", "visionquest", 2) {
		t.Errorf("unexpected match for completelydifferent")
	}
}
