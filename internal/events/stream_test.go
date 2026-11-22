package events

import "testing"

func TestStreamResumeRules(t *testing.T) {
	s := NewStream[int]("e1", 4)
	for i := 1; i <= 6; i++ {
		s.Append(i * 10)
	}
	cases := []struct {
		name  string
		epoch string
		pos   uint64
		ok    bool
		want  []int
	}{
		{"caught up", "e1", 6, true, []int{}},
		{"two behind", "e1", 4, true, []int{50, 60}},
		{"oldest kept", "e1", 2, true, []int{30, 40, 50, 60}},
		{"too old", "e1", 1, false, nil},
		{"ahead", "e1", 7, false, nil},
		{"new epoch", "e0", 6, false, nil},
		{"empty epoch", "", 0, false, nil},
	}
	for _, c := range cases {
		got, ok := s.Resume(c.epoch, c.pos)
		if ok != c.ok || len(got) != len(c.want) {
			t.Errorf("%s: got %v %v", c.name, got, ok)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v", c.name, got)
			}
		}
	}
}
