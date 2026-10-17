package runner

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestPSArgsEncodedCommand(t *testing.T) {
	args := psArgs(`Write-Host "olá"`)
	if args[len(args)-2] != "-EncodedCommand" {
		t.Fatalf("args = %v", args)
	}
	raw, err := base64.StdEncoding.DecodeString(args[len(args)-1])
	if err != nil || len(raw)%2 != 0 {
		t.Fatal(err)
	}
	u := make([]uint16, len(raw)/2)
	for i := range u {
		u[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	script := string(utf16.Decode(u))
	if !strings.HasPrefix(script, psPrelude) || !strings.HasSuffix(script, `Write-Host "olá"`) {
		t.Errorf("script = %q", script)
	}
}

func TestPSQuote(t *testing.T) {
	if got := PSQuote(`C:\Users\D'Ávila`); got != `'C:\Users\D''Ávila'` {
		t.Errorf("PSQuote = %s", got)
	}
}

func TestDescribeMultilinha(t *testing.T) {
	got := describe("powershell", []string{"-c", "a\n  # comentário\n b\n"})
	if got != "powershell -c 'a; b'" {
		t.Errorf("describe = %q", got)
	}
}
