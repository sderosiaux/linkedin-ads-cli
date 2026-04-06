// Package confirm provides an interactive Y/N prompt used by every
// destructive command in the CLI. The prompt is intentionally tiny so the
// CLI's write paths can stay testable.
package confirm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Prompt writes msg followed by " [y/N]: " to out, reads a single line from
// in, and returns true when the answer starts with y or Y. EOF is treated as
// a no.
func Prompt(in io.Reader, out io.Writer, msg string) (bool, error) {
	if _, err := fmt.Fprintf(out, "%s [y/N]: ", msg); err != nil {
		return false, err
	}
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	answer := strings.TrimSpace(line)
	return strings.HasPrefix(strings.ToLower(answer), "y"), nil
}
