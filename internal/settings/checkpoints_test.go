package settings

import (
	"encoding/json"
	"testing"
)

func TestCheckpointsOffIsPerProjectAndDefaultsOn(t *testing.T) {
	s, _, _ := newService(t)
	if got := s.Get().CheckpointsOff; got == nil || len(got) != 0 {
		t.Fatalf("default = %v", got)
	}
	if _, err := s.Save(json.RawMessage(`{"checkpointsOff":{"p1":true}}`)); err != nil {
		t.Fatal(err)
	}
	if got := s.Get().CheckpointsOff; !got["p1"] || got["p2"] {
		t.Fatalf("saved = %v", got)
	}
	if _, err := s.Save(json.RawMessage(`{"titleJobs":true}`)); err != nil || !s.Get().CheckpointsOff["p1"] {
		t.Fatalf("another key changed it: %v %v", err, s.Get().CheckpointsOff)
	}
	if _, err := s.Save(json.RawMessage(`{"checkpointsOff":{}}`)); err != nil || len(s.Get().CheckpointsOff) != 0 {
		t.Fatalf("clear: %v %v", err, s.Get().CheckpointsOff)
	}
}
