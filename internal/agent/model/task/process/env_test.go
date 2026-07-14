package process

import (
	"reflect"
	"testing"
)

func TestSanitizeLaunchEnvStripsShellOptionVars(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"SHELLOPTS=braceexpand:errexit:hashall",
		"HOME=/home/user",
		"bashopts=checkwinsize:cmdhist",
		"TERM=xterm-256color",
		"BASHOPTS=extglob:histappend",
		"EMPTY_VALUE=",
		"NO_EQUALS_SIGN",
	}
	got := SanitizeLaunchEnv(input)
	want := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"TERM=xterm-256color",
		"EMPTY_VALUE=",
		"NO_EQUALS_SIGN",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeLaunchEnv() = %#v, want %#v", got, want)
	}
}

func TestSanitizeLaunchEnvPreservesNil(t *testing.T) {
	if got := SanitizeLaunchEnv(nil); got != nil {
		t.Fatalf("SanitizeLaunchEnv(nil) = %#v, want nil", got)
	}
}

func TestSanitizeLaunchEnvPreservesEmpty(t *testing.T) {
	got := SanitizeLaunchEnv([]string{})
	if got == nil || len(got) != 0 {
		t.Fatalf("SanitizeLaunchEnv([]) = %#v, want empty non-nil slice", got)
	}
}
