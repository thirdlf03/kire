package main

import (
	"testing"
)

func TestResolveNegatables(t *testing.T) {
	val := true
	noVal := true
	saved := negatables
	defer func() { negatables = saved }()

	negatables = []negatableBool{{val: &val, noVal: &noVal}}
	resolveNegatables()

	if val {
		t.Error("expected val to be false after resolveNegatables with noVal=true")
	}
}

func TestResolveNegatables_NoOverride(t *testing.T) {
	val := true
	noVal := false
	saved := negatables
	defer func() { negatables = saved }()

	negatables = []negatableBool{{val: &val, noVal: &noVal}}
	resolveNegatables()

	if !val {
		t.Error("expected val to remain true when noVal=false")
	}
}
