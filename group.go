// Copyright (c) amoeba Authors. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package amoeba

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/revzim/amoeba/internal/env"
	"github.com/revzim/amoeba/internal/log"
	"github.com/revzim/amoeba/internal/message"
	"github.com/revzim/amoeba/scheduler"
	"github.com/revzim/amoeba/session"
	"github.com/revzim/azdrivers"
)

const (
	groupStatusWorking = 0
	groupStatusClosed  = 1
)

type (

	// SessionFilter represents a filter which was used to filter session when Multicast,
	// the session will receive the message while filter returns true.
	SessionFilter func(*session.Session) bool

	LifeCycles struct {
		OnUpdate *scheduler.GameLoop
	}

	// Group represents a session group which used to manage a number of
	// sessions, data send to the group will send to all session in it.
	Group struct {
		sync.RWMutex
		status   int32                      // channel current status
		name     string                     // channel name
		sessions map[int64]*session.Session // session id map to session instance

		onUpdate       func(float64)
		tickRate       time.Duration
		lifeCycles     *LifeCycles
		mongoDriver    *azdrivers.AZMongoApp
		firebaseDriver *azdrivers.AZFirebaseApp
	}
)

// NewGroup returns a new group instance
func NewGroup(n string) *Group {
	return &Group{
		status:     groupStatusWorking,
		name:       n,
		sessions:   make(map[int64]*session.Session),
		lifeCycles: &LifeCycles{},
	}
}

// NewGroupWithDriver --
// returns a new group instance with driver support
// driverType = mongo | firebase
// driverString = mongo uri | firebase cfg file path
func NewGroupWithDriver(n string, driverType azdrivers.DriverKeyType, newDriver bool, onUpdate func()) (*Group, error) {

	g := &Group{
		status:     groupStatusWorking,
		name:       n,
		sessions:   make(map[int64]*session.Session),
		lifeCycles: &LifeCycles{},
	}
	err := g.InitDriver(driverType, "", newDriver)
	if err != nil {
		return nil, err
	}
	// g.SetOnUpdate(onUpdate)
	return g, nil
}

func (g *Group) GetName() string {
	return g.name
}

// func (g *Group) GetTickRate() int64 {
// 	return int64(g.lifeCycles.OnUpdate.GetTickRate())
// }

func (g *Group) SetOnUpdate(fn func(delta float64), tickRate int64) {
	if g.lifeCycles.OnUpdate != nil {
		log.Println("stopping & gc OnUpdate method...")
		g.lifeCycles.OnUpdate.Stop()
		g.lifeCycles.OnUpdate = nil
		g.tickRate = time.Duration(0)
		g.onUpdate = nil
		log.Println("OnUpdate method stopped & gc")
	}
	g.tickRate = time.Duration(tickRate)
	g.onUpdate = fn
	g.lifeCycles.OnUpdate = scheduler.NewGameLoop(g.tickRate, g.onUpdate)
	go g.lifeCycles.OnUpdate.Start()
}

func (g *Group) GetOnUpdate() *scheduler.GameLoop {
	return g.lifeCycles.OnUpdate
}

func (g *Group) InitDriver(driverType azdrivers.DriverKeyType, driverString string, isNew bool) error {
	var err error
	log.Println(fmt.Sprintf("should init new %s driver? %v", driverType, isNew))
	switch driverType {
	case azdrivers.MongoKeyType:
		var mdriver *azdrivers.AZMongoApp
		if isNew {
			mdriver, err = azdrivers.NewMongoApp(driverString)
		} else {
			mdriver = env.MongoDriver
		}
		if err != nil {
			return err
		}
		g.mongoDriver = mdriver
	case azdrivers.FirebaseKeyType:
		var fdriver *azdrivers.AZFirebaseApp
		if isNew {
			fdriver, err = azdrivers.NewFirebaseApp(driverString)
			if err != nil {
				return err
			}
		} else {
			fdriver = env.FirebaseDriver
		}
		g.firebaseDriver = fdriver
	}
	return nil
}

// Member returns specified UID's session
func (g *Group) Member(uid int64) (*session.Session, error) {
	g.RLock()
	defer g.RUnlock()

	for _, s := range g.sessions {
		if s.UID() == uid {
			return s, nil
		}
	}

	return nil, ErrMemberNotFound
}

// Member returns specified UID's session
func (g *Group) GetMember(uid int64) (*session.Session, error) {
	g.RLock()
	defer g.RUnlock()
	return g.sessions[uid], ErrMemberNotFound
}

// Member returns specified UUID's session
func (g *Group) MemberUUID(uuid string) (*session.Session, error) {
	g.RLock()
	defer g.RUnlock()

	for _, s := range g.sessions {
		if s.UUID() == uuid {
			return s, nil
		}
	}

	return nil, ErrMemberNotFound
}

// Members returns all member's UID in current group
func (g *Group) Members() []int64 {
	g.RLock()
	defer g.RUnlock()

	var members []int64
	for _, s := range g.sessions {
		members = append(members, s.UID())
	}

	return members
}

// MembersUUID --
func (g *Group) MembersUUID() []string {
	g.RLock()
	defer g.RUnlock()

	var members []string
	for _, s := range g.sessions {
		members = append(members, s.UUID())
	}

	return members
}

// MembersShortUUID
func (g *Group) MembersShortUUID() []string {
	g.RLock()
	defer g.RUnlock()

	var members []string
	for _, s := range g.sessions {
		members = append(members, s.ShortUUID())
	}

	return members
}

// Multicast  push  the message to the filtered clients
func (g *Group) Multicast(route string, v interface{}, filter SessionFilter) error {
	if g.isClosed() {
		return ErrClosedGroup
	}

	data, err := message.Serialize(v)
	if err != nil {
		return err
	}

	if env.Debug {
		log.Println(fmt.Sprintf("Multicast %s, Data=%+v", route, v))
	}

	g.RLock()
	defer g.RUnlock()

	for _, s := range g.sessions {
		if !filter(s) {
			continue
		}
		if err = s.Push(route, data); err != nil {
			log.Println(err.Error())
		}
	}

	return nil
}

// Broadcast push  the message(s) to  all members
func (g *Group) Broadcast(route string, v interface{}) error {
	if g.isClosed() {
		return ErrClosedGroup
	}

	data, err := message.Serialize(v)
	if err != nil {
		return err
	}

	if env.Debug {
		log.Println(fmt.Sprintf("Broadcast %s, Data=%+v", route, v))
	}

	g.RLock()
	defer g.RUnlock()

	for _, s := range g.sessions {
		if err = s.Push(route, data); err != nil {
			log.Println(fmt.Sprintf("Session push message error, ID=%d, UID=%d, Error=%s", s.ID(), s.UID(), err.Error()))
		}
	}

	return err
}

// Contains check whether a UID is contained in current group or not
func (g *Group) Contains(uid int64) bool {
	_, err := g.Member(uid)
	return err == nil
}

// Contains check whether a UUID is contained in current group or not
func (g *Group) ContainsUUID(uuid string) bool {
	_, err := g.MemberUUID(uuid)
	return err == nil
}

// Add add session to group
func (g *Group) Add(session *session.Session) error {
	if g.isClosed() {
		return ErrClosedGroup
	}

	if env.Debug {
		log.Println(fmt.Sprintf("Add session to group %s, ID=%d, UID=%d", g.name, session.ID(), session.UID()))
	}

	g.Lock()
	defer g.Unlock()

	id := session.ID()
	_, ok := g.sessions[session.ID()]
	if ok {
		return ErrSessionDuplication
	}

	g.sessions[id] = session
	return nil
}

// Leave remove specified UID related session from group
func (g *Group) Leave(s *session.Session) error {
	if g.isClosed() {
		return ErrClosedGroup
	}

	if env.Debug {
		log.Println(fmt.Sprintf("Remove session from group %s, UID=%d", g.name, s.UID()))
	}

	g.Lock()
	defer g.Unlock()

	delete(g.sessions, s.ID())
	return nil
}

// LeaveAll clear all sessions in the group
func (g *Group) LeaveAll() error {
	if g.isClosed() {
		return ErrClosedGroup
	}

	g.Lock()
	defer g.Unlock()

	g.sessions = make(map[int64]*session.Session)
	return nil
}

// Count get current member amount in the group
func (g *Group) Count() int {
	g.RLock()
	defer g.RUnlock()

	return len(g.sessions)
}

func (g *Group) isClosed() bool {
	if atomic.LoadInt32(&g.status) == groupStatusClosed {
		return true
	}
	return false
}

// Close destroy group, which will release all resource in the group
func (g *Group) Close() error {
	if g.isClosed() {
		return ErrCloseClosedGroup
	}

	atomic.StoreInt32(&g.status, groupStatusClosed)

	// release all reference
	g.sessions = make(map[int64]*session.Session)
	return nil
}
