package nats

import (
	"testing"

	natsio "github.com/nats-io/nats.go"
)

func TestNatsMsg_Data(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("returns underlying data", func(t *testing.T) {
		// Arrange
		raw := &natsio.Msg{Data: []byte("hello")}
		msg := NewNatsMsg(raw)

		// Act
		got := msg.Data()

		// Assert
		if string(got) != "hello" {
			t.Fatalf("expected hello got %q", string(got))
		}
	})
}

func TestNatsMsg_Delivered(t *testing.T) {
	// Covers [SPEC-001: BR-003]
	t.Run("returns 1 when metadata error", func(t *testing.T) {
		// Arrange
		raw := &natsio.Msg{Data: []byte("x")}
		msg := NewNatsMsg(raw)

		// Act
		got := msg.Delivered()

		// Assert
		if got != 1 {
			t.Fatalf("expected 1 got %d", got)
		}
	})

	t.Run("NewNatsMsg wraps correctly", func(t *testing.T) {
		// Arrange
		raw := &natsio.Msg{Data: []byte("payload"), Subject: "telemetry.raw.ABC123"}

		// Act
		msg := NewNatsMsg(raw)

		// Assert
		if msg == nil {
			t.Fatalf("expected non-nil")
		}
		if string(msg.Data()) != "payload" {
			t.Fatalf("expected payload")
		}
	})
}

func TestNatsMsg_AckTermNak_without_connection_returns_error_not_panic(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("Ack returns error when no connection", func(t *testing.T) {
		// Arrange
		raw := &natsio.Msg{Data: []byte("x")}
		msg := NewNatsMsg(raw)

		// Act
		err := msg.Ack()

		// Assert
		if err == nil {
			t.Fatalf("expected error for ack without conn")
		}
	})

	t.Run("Nak returns error", func(t *testing.T) {
		// Arrange
		raw := &natsio.Msg{Data: []byte("x")}
		msg := NewNatsMsg(raw)

		// Act
		err := msg.Nak()

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("Term returns error", func(t *testing.T) {
		// Arrange
		raw := &natsio.Msg{Data: []byte("x")}
		msg := NewNatsMsg(raw)

		// Act
		err := msg.Term()

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("NakWithDelay returns error", func(t *testing.T) {
		// Arrange
		raw := &natsio.Msg{Data: []byte("x")}
		msg := NewNatsMsg(raw)

		// Act
		err := msg.NakWithDelay(1000000000)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
