package autostart

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func plist(e Env, o Options) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + label + `</string>
	<key>ProgramArguments</key>
	<array>
`)
	for _, a := range o.args(e.Exe) {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", esc(a))
	}
	b.WriteString("\t</array>\n\t<key>EnvironmentVariables</key>\n\t<dict>\n")
	if o.PathEnv != "" {
		fmt.Fprintf(&b, "\t\t<key>PATH</key>\n\t\t<string>%s</string>\n", esc(o.PathEnv))
	}
	if o.ShellEnv != "" {
		fmt.Fprintf(&b, "\t\t<key>SHELL</key>\n\t\t<string>%s</string>\n", esc(o.ShellEnv))
	}
	logPath := esc(o.Data + "/logs/autostart.log")
	b.WriteString(`	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>ProcessType</key>
	<string>Interactive</string>
	<key>StandardOutPath</key>
	<string>` + logPath + `</string>
	<key>StandardErrorPath</key>
	<string>` + logPath + `</string>
</dict>
</plist>
`)
	return b.String()
}

// parsePlist returns the ProgramArguments strings.
func parsePlist(text string) []string {
	dec := xml.NewDecoder(strings.NewReader(text))
	var out []string
	var key string
	inArgs, inKey := false, false
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			inKey = t.Name.Local == "key"
		case xml.EndElement:
			if t.Name.Local == "array" {
				inArgs = false
			}
		case xml.CharData:
			s := string(t)
			switch {
			case inKey:
				key = s
				inKey = false
				inArgs = key == "ProgramArguments"
			case inArgs && strings.TrimSpace(s) != "":
				out = append(out, s)
			}
		}
	}
}

func (e Env) domain() string { return fmt.Sprintf("gui/%d", e.UID) }

func (e Env) startLaunchd(file string) error {
	// Not loaded yet is fine.
	_, _ = e.Run("launchctl", "bootout", e.domain()+"/"+label)
	if out, err := e.Run("launchctl", "bootstrap", e.domain(), file); err != nil {
		return fmt.Errorf("start the job with launchctl: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
