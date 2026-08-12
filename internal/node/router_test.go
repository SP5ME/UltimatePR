package node

import "testing"

func TestResolveBestStaticRoute(t *testing.T) {
	r := New([]Neighbor{{ID: "a", Callsign: "SP5AAA-7", Port: "p1", Quality: 100}, {ID: "b", Callsign: "SP5BBB-7", Port: "p2", Quality: 200}}, []Route{{Destination: "SR5DDD-7", Via: "a", Quality: 80}, {Destination: "SR5DDD-7", Via: "b", Quality: 190}}, nil)
	n, _, e := r.Resolve("sr5ddd-7")
	if e != nil {
		t.Fatal(e)
	}
	if n.ID != "b" {
		t.Fatalf("neighbor=%s", n.ID)
	}
}
