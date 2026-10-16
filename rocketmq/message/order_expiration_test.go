package message

import "testing"

func TestOrderExpirationMessageRoundTrip(t *testing.T) {
	want, err := NewOrderExpirationMessage("12345")
	if err != nil {
		t.Fatal(err)
	}
	body, err := want.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseOrderMessage(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != ActionExpire || got.OrderID != want.OrderID {
		t.Fatalf("expiration event mismatch: got=%+v want=%+v", got, want)
	}
}
