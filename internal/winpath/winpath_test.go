package winpath

import "testing"

func TestJoin(t *testing.T) {
	for _, c := range []struct {
		elem []string
		want string
	}{
		{[]string{`C:\Users\ana`, "Projects"}, `C:\Users\ana\Projects`},
		{[]string{`D:\`, "Projects"}, `D:\Projects`},
		{[]string{`C:\`, "DevDrive", "DevDrive.vhdx"}, `C:\DevDrive\DevDrive.vhdx`},
		{[]string{`C:\x\`, `..\y`}, `C:\y`},
		{[]string{`D:\`}, `D:\`},
		{[]string{"", `C:\a`, ""}, `C:\a`},
	} {
		if got := Join(c.elem...); got != c.want {
			t.Errorf("Join(%q) = %q, esperado %q", c.elem, got, c.want)
		}
	}
}

func TestBaseDirExt(t *testing.T) {
	if got := Base(`C:\OVPN\empresa.ovpn`); got != "empresa.ovpn" {
		t.Errorf("Base = %q", got)
	}
	if got := Base(`C:\OVPN\`); got != "OVPN" {
		t.Errorf("Base com barra final = %q", got)
	}
	if got := Dir(`C:\php\php.exe`); got != `C:\php` {
		t.Errorf("Dir = %q", got)
	}
	if got := Dir(`C:\php.exe`); got != `C:\` {
		t.Errorf("Dir na raiz = %q", got)
	}
	if got := Ext(`C:\a.b\paisagem.JPG`); got != ".JPG" {
		t.Errorf("Ext = %q", got)
	}
}

func TestRelIgual(t *testing.T) {
	if r, ok := Rel(`C:\Clone\oh-my-posh`, `c:\clone\OH-MY-POSH\temas\a.json`); !ok || r != `temas\a.json` {
		t.Errorf("Rel = %q, %v", r, ok)
	}
	if _, ok := Rel(`C:\Clone\oh`, `C:\Clone\ohmy\a`); ok {
		t.Error("Rel aceitou um prefixo que não é pasta")
	}
	if !Igual(`C:\Users\Ana\Projects\`, `c:\users\ana\projects`) || Igual(`C:\a`, `C:\b`) {
		t.Error("Igual errado")
	}
}
