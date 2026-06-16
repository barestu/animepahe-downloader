package kwik

import (
	"strings"
	"testing"
)

func TestUnpack(t *testing.T) {
	// Classic Dean-Edwards packed sample: prints "hello world".
	packed := `eval(function(p,a,c,k,e,d){e=function(c){return c};` +
		`while(c--){if(k[c]){p=p.replace(new RegExp('\\b'+e(c)+'\\b','g'),k[c])}}` +
		`return p}('0 1',2,2,'hello|world'.split('|'),0,{}))`

	got, err := Unpack(packed)
	if err != nil {
		t.Fatalf("Unpack error: %v", err)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("unpacked = %q, want it to contain hello world", got)
	}
}

func TestUnpackNoPayload(t *testing.T) {
	if _, err := Unpack("not packed at all"); err == nil {
		t.Fatal("expected error for non-packed input")
	}
}

func TestUnbaserBase10(t *testing.T) {
	u := newUnbaser(10)
	if got := u.unbase("42"); got != 42 {
		t.Fatalf("unbase(42)=%d, want 42", got)
	}
}
