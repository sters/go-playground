package boilerplate

import "testing"

func TestHello(t *testing.T) {
	if want, got := "Hello", Hello(); want != got {
		t.Errorf("want = %s, god = %s", want, got)
	}
}
