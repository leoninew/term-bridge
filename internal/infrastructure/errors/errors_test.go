package errors

import "testing"

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: ExitSuccess},
		{name: "usage", err: Usage("bad args"), want: ExitUsage},
		{name: "config", err: Config("bad config", nil), want: ExitConfig},
		{name: "internal", err: Internal("bad internal", nil), want: ExitGeneral},
		{name: "runtime", err: Runtime("bad runtime", nil), want: ExitGeneral},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCode(tc.err); got != tc.want {
				t.Fatalf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}
