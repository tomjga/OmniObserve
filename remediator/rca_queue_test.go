package main

import "testing"

func TestRCAQueueDropsWhenFull(t *testing.T) {
	q := NewRCAQueue(1, 1)
	alert := Alert{
		Status:      "firing",
		Labels:      map[string]string{"alertname": "HighErrorRate", "service": "cart"},
		Annotations: map[string]string{"summary": "cart is failing"},
	}

	if !q.Enqueue(alert, "disabled flag") {
		t.Fatal("first enqueue should fit in the queue")
	}
	if q.Enqueue(alert, "disabled flag") {
		t.Fatal("second enqueue should drop when the queue is full")
	}
}
