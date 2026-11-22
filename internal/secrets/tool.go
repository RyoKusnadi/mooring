package secrets

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// secretTool is the Secret Service on Linux, through secret-tool. The value
// goes in on the standard input.
type secretTool struct {
	bin string
	run runner
}

func newSecretTool(bin string, r runner) *secretTool { return &secretTool{bin: bin, run: r} }

func (s *secretTool) Kind() string { return "secret-tool" }

// usable is true when a Secret Service answers: a lookup of a name that is not there exits with 1.
func (s *secretTool) usable() bool {
	_, code, err := s.run(s.bin, "", "lookup", "service", service, "name", "-probe")
	return err == nil && code == 1
}

func (s *secretTool) Get(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	out, code, err := s.run(s.bin, "", "lookup", "service", service, "name", name)
	if err != nil {
		return "", fmt.Errorf("run secret-tool: %w", err)
	}
	if code == 1 && len(out) == 0 {
		return "", ErrNotFound
	}
	if code != 0 {
		return "", fmt.Errorf("read the secret service: secret-tool exited with %d", code)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("read the secret service: the stored value is not one Mooring wrote")
	}
	return string(raw), nil
}

func (s *secretTool) Set(name, value string) error {
	if err := checkName(name); err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString([]byte(value))
	_, code, err := s.run(s.bin, enc, "store", "--label=Mooring "+name, "service", service, "name", name)
	if err != nil {
		return fmt.Errorf("run secret-tool: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("write the secret service: secret-tool exited with %d", code)
	}
	return nil
}

func (s *secretTool) Delete(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	_, code, err := s.run(s.bin, "", "clear", "service", service, "name", name)
	if err != nil {
		return fmt.Errorf("run secret-tool: %w", err)
	}
	if code != 0 && code != 1 {
		return fmt.Errorf("delete from the secret service: secret-tool exited with %d", code)
	}
	return nil
}
