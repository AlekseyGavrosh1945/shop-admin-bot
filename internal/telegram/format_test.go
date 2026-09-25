package telegram

import "testing"

func TestFmtInt(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1 000"},
		{1234567, "1 234 567"},
		{-4200, "-4 200"},
	}
	for _, c := range cases {
		if got := fmtInt(c.in); got != c.want {
			t.Errorf("fmtInt(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFmtEUR(t *testing.T) {
	if got := fmtEUR(1849.4); got != "1 849 €" {
		t.Errorf("fmtEUR = %q", got)
	}
}

func TestFmtQtyAndPrice(t *testing.T) {
	qty := int64(3)
	if got := fmtQty(nil); got != "—" {
		t.Errorf("fmtQty(nil) = %q", got)
	}
	if got := fmtQty(&qty); got != "3" {
		t.Errorf("fmtQty = %q", got)
	}
	price := 249.0
	if got := fmtPrice(nil); got != "—" {
		t.Errorf("fmtPrice(nil) = %q", got)
	}
	if got := fmtPrice(&price); got != "249 €" {
		t.Errorf("fmtPrice = %q", got)
	}
}

func TestEsc(t *testing.T) {
	if got := esc(`HP 2,5" SFF <new>`); got != `HP 2,5" SFF &lt;new&gt;` {
		t.Errorf("esc = %q", got)
	}
}
