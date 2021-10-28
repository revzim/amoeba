package logic

import (
	"log"

	"github.com/google/uuid"
	amoeba "github.com/revzim/amoeba"
	"github.com/revzim/amoeba/component"
	"github.com/revzim/amoeba/examples/demo/tadpole/logic/protocol"
	"github.com/revzim/amoeba/session"
)

// World contains all tadpoles
type World struct {
	component.Base
	*amoeba.Group
}

// NewWorld returns a world instance
func NewWorld() *World {
	return &World{
		Group: amoeba.NewGroup(uuid.New().String()),
	}
}

// Init initialize world component
func (w *World) Init() {
	session.Lifetime.OnClosed(func(s *session.Session) {
		w.Leave(s)
		w.Broadcast("leave", &protocol.LeaveWorldResponse{ID: s.ID()})
		log.Printf("session count: %d", w.Count())
	})
}

// Enter was called when new guest enter
func (w *World) Enter(s *session.Session, msg []byte) error {
	w.Add(s)
	log.Printf("session count: %d", w.Count())
	return s.Response(&protocol.EnterWorldResponse{ID: s.ID()})
}

// Update refresh tadpole's position
func (w *World) Update(s *session.Session, msg []byte) error {
	return w.Broadcast("update", msg)
}

// Message handler was used to communicate with each other
func (w *World) Message(s *session.Session, msg *protocol.WorldMessage) error {
	msg.ID = s.ID()
	return w.Broadcast("message", msg)
}
