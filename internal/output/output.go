package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type Printer struct {
	JSON bool
	Out  io.Writer
	Err  io.Writer
}

func New(jsonMode bool, out, err io.Writer) *Printer {
	return &Printer{JSON: jsonMode, Out: out, Err: err}
}

func (p *Printer) Print(value any) error {
	if p.JSON {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.Out, string(data))
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		_, err = fmt.Fprintln(p.Out, string(data))
		return err
	}
	_, err = fmt.Fprintln(p.Out, value)
	return err
}

func (p *Printer) Text(format string, args ...any) error {
	_, err := fmt.Fprintf(p.Out, format+"\n", args...)
	return err
}

func (p *Printer) Error(action string, err error) {
	if p.JSON {
		exitCode := 1
		var coded interface{ ExitCode() int }
		if errors.As(err, &coded) && coded.ExitCode() > 0 {
			exitCode = coded.ExitCode()
		}
		data := map[string]any{"ok": false, "action": action, "error": err.Error(), "exit_code": exitCode}
		_ = p.Print(data)
		return
	}
	_, _ = fmt.Fprintf(p.Err, "error: %s\n", err)
}
