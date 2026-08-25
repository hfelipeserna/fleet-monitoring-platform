package nats

import (
	"testing"

	natsio "github.com/nats-io/nats.go"
)

// Covers [SPEC-001: BR-003]

func TestCoverage_DLQDataAck(t *testing.T) {
	t.Run("dlqMsg Data and Ack", func(t *testing.T) {
		// Arrange
		msg := &natsio.Msg{Data: []byte("hello")}
		d := &dlqMsg{m: msg}

		// Act
		data := d.Data()
		err := d.Ack()

		// Assert
		if string(data) != "hello" {
			t.Fatalf("expected hello, got %q", string(data))
		}
		// Ack without server will return error (no connection), but should not panic
		if err == nil {
			t.Logf("Ack returned nil, acceptable without server")
		}
	})

	t.Run("NewDLQJetStream and helpers", func(t *testing.T) {
		// Arrange
		// Act
		d := NewDLQJetStream(nil)
		// Assert
		if d == nil {
			t.Fatalf("expected not nil")
		}
		if SanitizeDLQLimit(0) != DefaultDLQLimit {
			t.Fatalf("expected default")
		}
		if ResolveSubject("") != "telemetry.raw.unknown" {
			t.Fatalf("expected unknown")
		}
		if ResolveSubject("ABC123") != "telemetry.raw.ABC123" {
			t.Fatalf("expected ABC123")
		}
	})
}
