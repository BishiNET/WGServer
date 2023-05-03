package main

import "testing"

func TestGet(t *testing.T) {
	t.Log(GetUser("test"))
	t.Log(GetUser("test"))
	t.Log(GetAll())
}
