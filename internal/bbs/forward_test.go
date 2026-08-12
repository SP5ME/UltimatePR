package bbs

import (
	"path/filepath"
	"testing"
)

func TestPlanForwardingByAddressAndScope(t *testing.T) {
	ms := []Message{{ID: 1, Type: "P", To: "SQ5XYZ", At: "SR5DDD.#PL"}, {ID: 2, Type: "B", To: "POL"}, {ID: 3, Type: "B", To: "USA"}}
	ps := []ForwardPeer{{ID: "sr5ddd", Enabled: true, PrivateRoutes: []string{"SR5DDD.#PL", "#PL"}, BulletinScopes: []string{"POL"}}}
	got := PlanForwarding(ms, ps, 10)
	if len(got) != 2 {
		t.Fatalf("planned %d", len(got))
	}
	if got[0].Message.ID != 1 || got[1].Message.ID != 2 {
		t.Fatalf("wrong messages: %+v", got)
	}
}

func TestPrepareAndRecordForwardQueue(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "bbs.json"))
	m, err := s.Send("P", "SP5AAA", "SQ5XYZ@SR5DDD.#PL", "Test", "Body")
	if err != nil {
		t.Fatal(err)
	}
	peers := []ForwardPeer{{ID: "sr5ddd", Enabled: true, PrivateRoutes: []string{"#PL"}}}
	if err = PrepareQueues(s, peers, 10); err != nil {
		t.Fatal(err)
	}
	q := s.ForwardQueue("sr5ddd", 10)
	if len(q) != 1 || q[0].ID != m.ID {
		t.Fatalf("queue=%+v", q)
	}
	if err = s.RecordForward("sr5ddd", m.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if len(s.ForwardQueue("sr5ddd", 10)) != 0 {
		t.Fatal("delivered message still queued")
	}
}
