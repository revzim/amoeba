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

package scheduler

import "time"

type (
	Ticker struct {
		*time.Ticker
		ID       string        `json:"id"` // HELPER ID IF NEEDED
		Valid    bool          `json:"valid"`
		DoneChan chan bool     `json:"done,omitempty"`
		Duration time.Duration `json:"duration"`
		Start    int64         `json:"start"`
		End      int64         `json:"end"`
	}
)

func NewTicker(duration time.Duration, init bool) *Ticker {
	startTime := time.Now().UnixNano()

	// ticker := time.NewTicker(duration)
	tkr := &Ticker{
		Duration: duration,
		Start:    startTime,
		End:      startTime + int64(duration),
		Valid:    true,
		// Ticker:   ticker,
		DoneChan: make(chan bool, 1),
	}
	if init {
		tkr.Ticker = time.NewTicker(duration)
	}
	return tkr
}

// SetTimeout --
// FOLLOWS JS SetTimeout PARADIGM
func SetTimeout(tickerFn func(), duration time.Duration) *Ticker {
	ticker := NewTicker(duration, true)
	ticker.DoneChan <- true
	for range ticker.C {
		safecall(0, tickerFn)
		ticker.Clear()
		return ticker
	}
	return ticker
}

// SetInterval --
// FOLLOWS JS SetInterval PARADIGM
// DONE CHAN TO QUIT OUT OF INTERVAL
func SetInterval(done chan bool, tickerFn func(), duration time.Duration) *Ticker {
	ticker := NewTicker(duration, true)
	go func() {
		for {
			select {
			case <-ticker.C:
				safecall(0, tickerFn)
			case <-done:
				ticker.Clear()
				return
			}
		}
	}()
	return ticker
}

// Clear --
// CLEARS TICKER VALS
func (t *Ticker) Clear() {
	if t == nil {
		return
	}
	if len(t.DoneChan) == cap(t.DoneChan) {
		_ = <-t.DoneChan
	}
	t.Stop()
	t.Valid = false
}
