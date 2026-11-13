// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"io"
	"reflect"
	"testing"
)

func TestParseFlags(t *testing.T) {
	cases := []struct {
		args     []string
		bind     string
		stdin    bool
		wantArgs []string
	}{
		{[]string{"--bind", "U", "db", "f.kdbx"}, "U", false, []string{"db", "f.kdbx"}},
		{[]string{"db", "f.kdbx", "--bind", "U"}, "U", false, []string{"db", "f.kdbx"}},
		{[]string{"db", "--bind=U", "f.kdbx", "--password-stdin"}, "U", true, []string{"db", "f.kdbx"}},
		{[]string{"db", "--", "-odd-name"}, "", false, []string{"db", "-odd-name"}},
		{[]string{"--", "--bind", "x"}, "", false, []string{"--bind", "x"}},
		{nil, "", false, []string{}},
	}
	for _, c := range cases {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		bind := fs.String("bind", "", "")
		stdin := fs.Bool("password-stdin", false, "")
		if err := parseFlags(fs, c.args); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		got := fs.Args()
		if got == nil {
			got = []string{}
		}
		if *bind != c.bind || *stdin != c.stdin || !reflect.DeepEqual(got, c.wantArgs) {
			t.Errorf("%v: bind=%q stdin=%v args=%q, want %q %v %q", c.args, *bind, *stdin, got, c.bind, c.stdin, c.wantArgs)
		}
	}
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("bind", "", "")
	if err := parseFlags(fs, []string{"db", "--nope"}); err == nil {
		t.Error("unknown flag after positional args must still be an error")
	}
}
