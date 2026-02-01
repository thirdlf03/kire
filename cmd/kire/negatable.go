package main

import "github.com/spf13/cobra"

// negatableBool holds a pair of flags: --flag and --no-flag.
type negatableBool struct {
	val   *bool
	noVal *bool
}

var negatables []negatableBool

// negatable registers a boolean flag with a --no-* counterpart on the given command.
// The --no-* flag is hidden and, when set, overrides the positive flag to false.
func negatable(cmd *cobra.Command, name string, value bool, usage string) *bool {
	val := cmd.Flags().Bool(name, value, usage)
	noVal := cmd.Flags().Bool("no-"+name, false, "")
	_ = cmd.Flags().MarkHidden("no-" + name)
	negatables = append(negatables, negatableBool{val: val, noVal: noVal})
	return val
}

// resolveNegatables applies --no-* overrides: if --no-flag was set, *flag becomes false.
func resolveNegatables() {
	for _, n := range negatables {
		if *n.noVal {
			*n.val = false
		}
	}
}
