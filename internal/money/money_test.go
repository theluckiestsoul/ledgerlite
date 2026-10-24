package money

import (
	"errors"
	"testing"
)

func TestFormat(t *testing.T) {
	cases := map[int64]string{12345: "123.45 INR", 5: "0.05 INR", -1999: "-19.99 INR", 0: "0.00 INR"}
	for in, want := range cases {
		if got := Format(in, INR); got != want {
			t.Errorf("Format(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestValidAmount(t *testing.T) {
	for _, v := range []int64{0, -1, MaxAmount + 1} {
		if ValidAmount(v) {
			t.Errorf("ValidAmount(%d) = true, want false", v)
		}
	}
	if !ValidAmount(1) || !ValidAmount(MaxAmount) {
		t.Error("expected 1 and MaxAmount to be valid")
	}
}

func TestAddOverflow(t *testing.T) {
	if _, err := Add(MaxAmount, 1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("Add overflow err = %v, want ErrOverflow", err)
	}
	if got, err := Add(2, 3); err != nil || got != 5 {
		t.Fatalf("Add(2, 3) = %d, %v", got, err)
	}
}

func TestParseCurrency(t *testing.T) {
	if c, ok := ParseCurrency("USD"); !ok || c != USD {
		t.Fatal("expected USD to parse")
	}
	if _, ok := ParseCurrency("BTC"); ok {
		t.Fatal("expected BTC to be rejected")
	}
}
