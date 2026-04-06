package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
)

// compactFn projects a value to a smaller representation. Used by writeOutput
// when --compact is set together with --json. The function is called per
// element when the input is a slice, otherwise once on the value itself.
type compactFn func(any) any

// writeOutput dispatches data to JSON or the terminal format function.
//
// If --limit > 0 and data is a slice, it is truncated to N items first. If
// --json is set, data is encoded as indented JSON; --compact additionally
// applies the optional compact projector when one is provided. Otherwise the
// terminal format function is invoked.
func writeOutput(cmd *cobra.Command, data any, terminalFn func() string, compact ...compactFn) error {
	data = applyLimit(cmd, data)

	jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
	if jsonFlag {
		compactFlag, _ := cmd.Root().PersistentFlags().GetBool("compact")
		if compactFlag && len(compact) > 0 && compact[0] != nil {
			data = applyCompact(data, compact[0])
		}
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(b)); err != nil {
			return err
		}
		return nil
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), terminalFn()); err != nil {
		return err
	}
	return nil
}

// limitFlag returns the value of the global --limit flag (0 when unset).
func limitFlag(cmd *cobra.Command) int {
	n, _ := cmd.Root().PersistentFlags().GetInt("limit")
	return n
}

// applyLimit truncates a slice to limitFlag(cmd) items. Non-slice values pass
// through unchanged.
func applyLimit(cmd *cobra.Command, data any) any {
	n := limitFlag(cmd)
	if n <= 0 {
		return data
	}
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice {
		return data
	}
	if v.Len() <= n {
		return data
	}
	return v.Slice(0, n).Interface()
}

// applyCompact projects each element of a slice through fn (or fn applied once
// when data is not a slice).
func applyCompact(data any, fn compactFn) any {
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice {
		return fn(data)
	}
	out := make([]any, v.Len())
	for i := 0; i < v.Len(); i++ {
		out[i] = fn(v.Index(i).Interface())
	}
	return out
}
