package winapi

import "testing"

func TestSplitKey(t *testing.T) {
	for in, want := range map[string][2]string{
		`HKCU\Software\X`:            {"HKCU", `Software\X`},
		`HKLM:\SOFTWARE\Policies\Y`:  {"HKLM", `SOFTWARE\Policies\Y`},
		`HKEY_CURRENT_USER\System\Z`: {"HKCU", `System\Z`},
	} {
		root, sub, err := SplitKey(in)
		if err != nil || root != want[0] || sub != want[1] {
			t.Errorf("SplitKey(%q) = %q, %q, %v", in, root, sub, err)
		}
	}
	for _, in := range []string{`HKCR\x`, `HKCU`, ``} {
		if _, _, err := SplitKey(in); err == nil {
			t.Errorf("SplitKey(%q) deveria falhar", in)
		}
	}
}

func TestQuoteArg(t *testing.T) {
	for in, want := range map[string]string{
		"simples":                   "simples",
		"":                          `""`,
		"com espaço":                `"com espaço"`,
		`C:\Program Files\`:         `"C:\Program Files\\"`,
		`/VERYSILENT /MERGETASKS=a`: `"/VERYSILENT /MERGETASKS=a"`,
		`diz "oi"`:                  `"diz \"oi\""`,
	} {
		if got := QuoteArg(in); got != want {
			t.Errorf("QuoteArg(%q) = %s, esperado %s", in, got, want)
		}
	}
}
