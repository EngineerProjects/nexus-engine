package bash

import "testing"

func TestLooksLikeAwaitingInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"python prompt", "Python 3.12.0\n>>> ", true},
		{"python continuation", "def foo():\n... ", true},
		{"node prompt", "Welcome to Node.js v20\n> ", true},
		{"ipython prompt", "In [3]: ", true},
		{"julia prompt", "julia> ", true},
		{"gdb prompt", "(gdb) ", true},
		{"mysql prompt", "mysql> ", true},
		{"psql prompt", "mydb=# ", true},
		{"generic shell dollar", "user@host:~$ ", true},
		{"password prompt", "Password: ", true},
		{"yn prompt", "Overwrite file? [y/N] ", true},
		{"mid output mention of prompt", "This script prints >>> as a marker\nand keeps going with more text after it that is not a prompt", false},
		{"plain program output", "Processing item 1 of 10\nDone with step A", false},
		{"empty output", "", false},
		{"trailing newline no prompt", "some output\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeAwaitingInput(tc.output); got != tc.want {
				t.Errorf("looksLikeAwaitingInput(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}
