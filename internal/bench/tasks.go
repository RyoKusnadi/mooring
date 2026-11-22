package bench

// part is one prompt of a task. dry is what the stand-in agent is sent instead.
type part struct {
	prompt string
	dry    string
}

// task is one thing the agent is asked to do in the fixture.
type task struct {
	name  string
	parts []part
}

const (
	fixPrompt    = "Run `go test ./...` and fix the failing test by changing the code, not the tests. Stop when every test passes."
	renamePrompt = "Rename the function ParseRow to ParseRecord everywhere in this module, tests included. Then run `go build ./...` to confirm it builds."
)

var allTasks = []task{
	{"fix-test", []part{{fixPrompt, "BENCH fix-test"}}},
	{"rename", []part{{renamePrompt, "BENCH rename"}}},
	{"add-flag", []part{{
		"Add a `-limit N` flag to cmd/tally. With N above 0 it prints only the first N lines of its output; the default 0 prints all of them. Try it with `go run ./cmd/tally -limit 3 < testdata/in.txt`.",
		"BENCH add-flag"}}},
	{"find-log", []part{{
		"Run `make noisy`. It prints about 12000 lines and one of them reports a real test failure. Write the import path of the failing package, and nothing else, to ANSWER.txt.",
		"BENCH find-log"}}},
	{"continue", []part{
		{fixPrompt, "BENCH continue.1"},
		{renamePrompt + " Run `go test ./...` once at the end.", "BENCH continue.2"}}},
}

// TaskNames lists the task names in run order.
func TaskNames() []string {
	out := make([]string, len(allTasks))
	for i, t := range allTasks {
		out[i] = t.name
	}
	return out
}

func findTask(name string) (task, bool) {
	for _, t := range allTasks {
		if t.name == name {
			return t, true
		}
	}
	return task{}, false
}
