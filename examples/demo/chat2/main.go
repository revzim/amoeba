// https://github.com/revzim/amoeba/examples/chat2
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	amoeba "github.com/revzim/amoeba"
	"github.com/revzim/amoeba/auth"
	"github.com/revzim/amoeba/component"
	"github.com/revzim/amoeba/pipeline"
	"github.com/revzim/amoeba/scheduler"
	gsjson "github.com/revzim/amoeba/serialize/json"
	"github.com/revzim/amoeba/session"
)

type (
	Room struct {
		group *amoeba.Group
	}

	// RoomManager represents a component that contains a bundle of room
	RoomManager struct {
		component.Base
		timer *scheduler.Timer
		rooms map[int]*Room
	}

	// UserMessage represents a message that user sent
	UserMessage struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

	// NewUser message will be received when new user join room
	NewUser struct {
		Content string `json:"content"`
	}

	// AllMembers contains all members uid
	AllMembers struct {
		Members []string /*[]int64*/ `json:"members"`
	}

	// JoinResponse represents the result of joining room
	JoinResponse struct {
		Code     int    `json:"code"`
		Result   string `json:"result"`
		Username string `json:"username"`
	}

	stats struct {
		component.Base
		timer         *scheduler.Timer
		outboundBytes int
		inboundBytes  int
	}
)

const (
	port = 8080
)

func (stats *stats) outbound(s *session.Session, msg *pipeline.Message) error {
	stats.outboundBytes += len(msg.Data)
	return nil
}

func (stats *stats) inbound(s *session.Session, msg *pipeline.Message) error {
	stats.inboundBytes += len(msg.Data)
	return nil
}

func (stats *stats) AfterInit() {
	stats.timer = scheduler.NewTimer(time.Minute, func() {
		println("OutboundBytes", stats.outboundBytes)
		println("InboundBytes", stats.outboundBytes)
	})
}

const (
	testRoomID = 1
	roomIDKey  = "ROOM_ID"
)

func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: map[int]*Room{},
	}
}

// AfterInit component lifetime callback
func (mgr *RoomManager) AfterInit() {
	session.Lifetime.OnClosed(func(s *session.Session) {
		if !s.HasKey(roomIDKey) {
			return
		}
		room := s.Value(roomIDKey).(*Room)
		room.group.Leave(s)
	})
	mgr.timer = scheduler.NewTimer(time.Minute, func() {
		for roomId, room := range mgr.rooms {
			println(fmt.Sprintf("UserCount: RoomID=%d, Time=%s, Count=%d",
				roomId, time.Now().String(), room.group.Count()))
		}
	})
}

// Join room
func (mgr *RoomManager) Join(s *session.Session, msg []byte) error {
	// NOTE: join test room only in demo
	// log.Println(string(msg))
	room, found := mgr.rooms[testRoomID]
	if !found {
		room = &Room{
			group: amoeba.NewGroup(fmt.Sprintf("room-%d", testRoomID)),
		}
		// g := amoeba.NewGroup(fmt.Sprintf("room-%d", testRoomID))
		// if err != nil {
		// 	return err
		// }
		// room = &Room{
		// 	group: g, // amoeba.NewGroup(fmt.Sprintf("room-%d", testRoomID)),
		// }

		// SET ON UPDATE POST ROOM INIT
		roomOnUpdate := func(delta float64) {
			if room.group.Count() > 0 {
				err := room.group.Broadcast("onMessage", &UserMessage{
					Name:    "server",
					Content: fmt.Sprintf("%s tick rate: %dHz | interval: %dms", room.group.GetName(), room.group.GetOnUpdate().GetTickRate(), room.group.GetOnUpdate().GetTickMS()),
				})
				if err != nil {
					log.Println(err)
				}
			}
			log.Println(fmt.Sprintf("%s clients: %d", room.group.GetName(), room.group.Count()))
		}
		room.group.SetOnUpdate(roomOnUpdate, 30)
		mgr.rooms[testRoomID] = room
	}

	fakeUID := s.ID() //just use s.ID as uid !!!
	// uid := uuid.New().String()[:6]
	s.Bind(fakeUID) // binding session uids.Set(roomIDKey, room)
	s.Set(roomIDKey, room)
	s.Set(fmt.Sprintf("%d", fakeUID), s.ShortUUID())
	// log.Printf("%s", s.UUID())
	// s.Push("onMembers", &AllMembers{Members: room.group.MembersShortUUID()}) // uncomment if using json serializer
	b, _ := json.Marshal(AllMembers{Members: room.group.MembersShortUUID()})
	s.Push("onMembers", b)
	// notify others
	room.group.Broadcast("onNewUser", &NewUser{Content: fmt.Sprintf("New user: %s", s.ShortUUID())})
	// new user join group
	room.group.Add(s) // add session to group
	// b, _ := jsonn.Marshal(JoinResponse{Result: "success", Username: uid})
	// return s.Response(&JoinResponse{Result: "success", Username: s.ShortUUID()}) // uncomment if using json serializer
	b2, _ := json.Marshal(JoinResponse{Result: "success", Username: s.ShortUUID()})
	return s.Response(b2)
}

// Message sync last message to all members
func (mgr *RoomManager) Message(s *session.Session, msg *UserMessage) error {
	if !s.HasKey(roomIDKey) {
		return fmt.Errorf("not join room yet")
	}
	room := s.Value(roomIDKey).(*Room)
	return room.group.Broadcast("onMessage", msg)
}

func main() {
	components := &component.Components{}
	components.Register(
		NewRoomManager(),
		component.WithName("room"), // rewrite component and handler name
		component.WithNameFunc(strings.ToLower),
	)

	// traffic stats
	pip := pipeline.New()
	var stats = &stats{}
	pip.Outbound().PushBack(stats.outbound)
	pip.Inbound().PushBack(stats.inbound)

	log.SetFlags(log.LstdFlags | log.Llongfile)
	// http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("web"))))
	amoebaJWT := auth.NewJWT("TESTJWTKEY", jwt.SigningMethodHS256.Name, nil)

	tknStr, _ := amoebaJWT.GenerateToken(map[string]interface{}{
		"id":   "super user",
		"name": "awesome man",
		"cid":  uuid.New().String(),
	}, 1800)
	log.Println(tknStr)

	amoeba.Listen(fmt.Sprintf(":%d", port),
		amoeba.WithIsWebsocket(true),
		amoeba.WithJWT(amoebaJWT),
		// amoeba.WithJWTOpts("TESTJWTKEY", jwt.SigningMethodHS256.Name, nil),
		// amoeba.WithMongo(os.Getenv("MONGO_URI")),
		// amoeba.WithFirebase(os.Getenv("FIREBASE_CFG")),
		amoeba.WithHandshakeValidator(func(dataBytes []byte) error {
			// log.Println("handshake validator: ", dataBytes)
			return nil
		}),
		amoeba.WithPipeline(pip),
		amoeba.WithCheckOriginFunc(func(_ *http.Request) bool { return true }),
		amoeba.WithWSPath("/ws"),
		amoeba.WithDebugMode(),
		amoeba.WithSerializer(gsjson.NewSerializer()), // override default serializer
		amoeba.WithComponents(components),
	)
}
