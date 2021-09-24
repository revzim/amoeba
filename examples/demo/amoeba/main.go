// https://github.com/revzim/amoeba/examples/chat
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
	"github.com/revzim/amoeba"
	"github.com/revzim/amoeba/auth"
	"github.com/revzim/amoeba/component"
	"github.com/revzim/amoeba/crypt"
	"github.com/revzim/amoeba/pipeline"
	"github.com/revzim/amoeba/scheduler"
	gsjson "github.com/revzim/amoeba/serialize/json"
	"github.com/revzim/amoeba/session"
	"github.com/revzim/azdrivers"
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
	testRoomID = 1
	roomIDKey  = "ROOM_ID"
	port       = 80
)

var (
	gsCrypt   = crypt.New([]byte(""))
	amoebaJWT *auth.JWT
)

func (stats *stats) outbound(s *session.Session, msg *pipeline.Message) error {
	stats.outboundBytes += len(msg.Data)
	msg.Data, _ = gsCrypt.Encrypt(msg.Data)
	// msg.Data = []byte(crypt.Decode(string(msg.Data)))
	return nil
}

func (stats *stats) inbound(s *session.Session, msg *pipeline.Message) error {
	stats.inboundBytes += len(msg.Data)
	msg.Data, _ = gsCrypt.Decrypt(msg.Data)
	// msg.Data = []byte(crypt.Encode(string(msg.Data)))
	return nil
}

func (stats *stats) AfterInit() {
	stats.timer = scheduler.NewTimer(time.Minute, func() {
		println("OutboundBytes", stats.outboundBytes)
		println("InboundBytes", stats.outboundBytes)
	})
}

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

func quickEncrypt(str string) string {
	encryptedMsg, _ := gsCrypt.Encrypt([]byte(str))
	msg := crypt.Encode(string(encryptedMsg))
	return msg
}

// Join room
func (mgr *RoomManager) Join(s *session.Session, msg []byte) error {
	// NOTE: join test room only in demo
	// log.Println(string(msg))
	room, found := mgr.rooms[testRoomID]
	if !found {
		g, err := amoeba.NewGroupWithDriver(fmt.Sprintf("room-%d", testRoomID), azdrivers.FirebaseKeyType, false, nil)
		if err != nil {
			return err
		}
		room = &Room{
			group: g, // amoeba.NewGroup(fmt.Sprintf("room-%d", testRoomID)),
		}

		// SET ON UPDATE POST ROOM INIT
		roomOnUpdate := func(delta float64) {
			if room.group.Count() > 0 {
				updateMsg := fmt.Sprintf("%s tick rate: %dHz | interval: %dms", room.group.GetName(), room.group.GetOnUpdate().GetTickRate(), room.group.GetOnUpdate().GetTickMS())
				// msg := quickEncrypt(updateMsg)
				b, _ := json.Marshal(UserMessage{
					Name:    "server",
					Content: updateMsg,
				})
				err := room.group.Broadcast("onMessage", b)
				if err != nil {
					log.Println(err)
				}
			}
			// log.Println(fmt.Sprintf("%s clients: %d", room.group.GetName(), room.group.Count()))
		}
		// INTERVAL = 1 SECOND / TICKRATE
		room.group.SetOnUpdate(roomOnUpdate, 1)

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
	// s.Push("onMembers", &AllMembers{Members: room.group.MembersShortUUID()})
	// notify others
	newUserContent := fmt.Sprintf("New user: %s", s.ShortUUID())
	b1, _ := json.Marshal(NewUser{Content: newUserContent})
	// room.group.Broadcast("onNewUser", &NewUser{Content: crypt.Encode(newUserContent)})
	room.group.Broadcast("onNewUser", b1)
	// new user join group
	room.group.Add(s) // add session to group
	b2, _ := json.Marshal(JoinResponse{Result: "success", Username: s.ShortUUID()})
	return s.Response(b2)
	// return s.Response(&JoinResponse{Result: "success", Username: hash})
}

// Message sync last message to all members
func (mgr *RoomManager) Message(s *session.Session, data []byte) error {
	if !s.HasKey(roomIDKey) {
		return fmt.Errorf("not join room yet")
	}
	room := s.Value(roomIDKey).(*Room)

	var msg *UserMessage
	var err error
	var decryptedMsg []byte
	log.Println("on message: ", string(data))
	err = json.Unmarshal(data, &msg)
	if err != nil {
		log.Println("unmarshal err:", err)
		return room.group.Broadcast("onMessage", msg)
	}
	decryptedMsg, err = gsCrypt.Decrypt([]byte(crypt.Decode(msg.Content)))
	if err != nil {
		log.Println("DECRYPT ERROR:", err)
		return room.group.Broadcast("onMessage", msg)
	}
	// msg.Content = string(decryptedMsg)
	log.Println("DECRYPTED MSG: ", string(decryptedMsg))
	return room.group.Broadcast("onMessage", msg)
}

func InitGenericToken() string {
	tknStr, _ := amoebaJWT.GenerateToken(jwt.MapClaims{
		"id":   "super user",
		"name": "awesome man",
		"cid":  uuid.New().String(),
	}, 1800)
	return tknStr
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

	amoebaJWT = auth.NewJWT("JWTSIGNKEY", jwt.SigningMethodHS256.Name, nil)
	tknStr, _ := amoebaJWT.GenerateToken(jwt.MapClaims{
		"id":   "super user",
		"name": "awesome man",
		"cid":  uuid.New().String(),
	}, 1800)
	log.Println("\n", tknStr)

	// amoeba.Listen(fmt.Sprintf(":%d", port),
	amoeba.Listen(fmt.Sprintf(":%d", port),
		amoeba.WithIsWebsocket(true),
		amoeba.WithHandshakeValidator(func(dataBytes []byte) error {
			// log.Println("handshake validator: ", dataBytes)
			return nil
		}),
		amoeba.WithJWT(amoebaJWT),
		// amoeba.WithMongo(os.Getenv("MONGO_URI")),
		// amoeba.WithFirebase(os.Getenv("FIREBASE_CFG")),
		// amoeba.WithJWTOpts(string(server.GetJWTSignKey()), jwt.SigningMethodHS256.Name, server.NewJWTTokenString),
		amoeba.WithPipeline(pip),
		amoeba.WithCheckOriginFunc(func(_ *http.Request) bool { return true }),
		amoeba.WithWSPath("/ws"),
		amoeba.WithDebugMode(),
		amoeba.WithSerializer(gsjson.NewSerializer()), // override default serializer
		amoeba.WithComponents(components),
	)
}
