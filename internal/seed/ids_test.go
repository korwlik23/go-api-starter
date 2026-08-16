package seed

import "testing"

func TestIDsAreStableAndNamespaced(t *testing.T) {
	first := NewIDs()
	second := NewIDs()
	if first.Account != second.Account || first.OwnerUser != second.OwnerUser {
		t.Fatal("seed IDs must be stable")
	}
	if first.Account == first.OwnerUser {
		t.Fatal("seed records must have distinct IDs")
	}
	if first.User("author") == first.User("reviewer") {
		t.Fatal("user IDs must be namespaced by fixture key")
	}
	if first.Role("author") == first.Role("reviewer") {
		t.Fatal("role IDs must be namespaced by fixture key")
	}
}
