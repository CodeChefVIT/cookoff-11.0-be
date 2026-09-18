package utils

import "testing"

func signalled(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestNotifyWakesOnlyMatchingWaiters(t *testing.T) {
	a1, cancelA1 := WaitForResult("a")
	defer cancelA1()
	a2, cancelA2 := WaitForResult("a")
	defer cancelA2()
	b, cancelB := WaitForResult("b")
	defer cancelB()

	notifyResultWaiters("a")

	if !signalled(a1) || !signalled(a2) {
		t.Fatal("expected every waiter on a to be signalled")
	}
	if signalled(b) {
		t.Fatal("waiter on b must not be signalled by a")
	}
}

func TestRepeatedNotifyDoesNotBlock(t *testing.T) {
	ch, cancel := WaitForResult("dup")
	defer cancel()

	notifyResultWaiters("dup")
	notifyResultWaiters("dup") // buffer already full; must not block

	if !signalled(ch) {
		t.Fatal("expected the waiter to be signalled")
	}
}

func TestCancelRemovesWaiter(t *testing.T) {
	ch, cancel := WaitForResult("gone")
	cancel()

	notifyResultWaiters("gone")

	if signalled(ch) {
		t.Fatal("a cancelled waiter must not be signalled")
	}
	resultWaiters.Lock()
	_, stillTracked := resultWaiters.byID["gone"]
	resultWaiters.Unlock()
	if stillTracked {
		t.Fatal("expected the submission entry to be dropped once it has no waiters")
	}
}
