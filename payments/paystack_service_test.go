package payments

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"testing"
)

func TestToSubunit(t *testing.T) {
	cases := map[float64]int64{1500: 150000, 19.99: 1999, 0.1: 10, 1234.565: 123457}
	for in, want := range cases {
		if got := ToSubunit(in); got != want {
			t.Errorf("ToSubunit(%v) = %d, want %d", in, got, want)
		}
	}
}

func TestValidPaystackSignature(t *testing.T) {
	t.Setenv("PAYSTACK_SECRET_KEY", "sk_test_dummy")
	body := []byte(`{"event":"charge.success","data":{"reference":"abc"}}`)
	mac := hmac.New(sha512.New, []byte("sk_test_dummy"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !ValidPaystackSignature(body, sig) {
		t.Fatal("valid signature rejected")
	}
	if ValidPaystackSignature(body, "deadbeef") {
		t.Fatal("bad signature accepted")
	}
	if ValidPaystackSignature([]byte(`{"event":"charge.success"}`), sig) {
		t.Fatal("signature accepted for a different body")
	}
	if ValidPaystackSignature(body, "") {
		t.Fatal("empty signature accepted")
	}
}
