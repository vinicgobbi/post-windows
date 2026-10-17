package runlog

import "testing"

func TestProgresso(t *testing.T) {
	for line, want := range map[string]bool{
		"-": true, `\`: true, "  |  ": true, "/": true,
		"  ██████████▒▒▒▒▒▒▒▒  10.0 MB / 50.0 MB": true,
		"Found Git [Git.Git] Version 2.47.0":      false,
		"Successfully installed":                  false,
		"a":                                       false,
	} {
		if got := progresso(line); got != want {
			t.Errorf("progresso(%q) = %v", line, got)
		}
	}
}

func TestOutputWriterFiltraProgresso(t *testing.T) {
	l, _ := New("", "t")
	var got []string
	l.SetSink(func(e Entry) { got = append(got, e.Text) })
	w := l.OutputWriter()
	w.Write([]byte("Baixando\r-\r\\\r|\r  ████▒▒  1 MB / 2 MB\rInstalado\n"))
	w.Close()
	if len(got) != 2 || got[0] != "Baixando" || got[1] != "Instalado" {
		t.Errorf("got = %q", got)
	}
}
