package debug_test

import (
	"github.com/rubiojr/rapi/internal/debug"
	"github.com/rubiojr/rapi/restic"

	"testing"
)

func BenchmarkLogStatic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		debug.Log("Static string")
	}
}

func BenchmarkLogIDStr(b *testing.B) {
	id := restic.NewRandomID()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		debug.Log("id: %v", id)
	}
}

func BenchmarkLogIDString(b *testing.B) {
	id := restic.NewRandomID()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		debug.Log("id: %s", id)
	}
}
