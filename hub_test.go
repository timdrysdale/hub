package hub

import (
	"bytes"
	"crypto/rand"
	"reflect"
	"testing"
	"time"
)

func TestInstantiateHub(t *testing.T) {

	h := New()

	if reflect.TypeOf(h.Broadcast) != reflect.TypeOf(make(chan Message)) {
		t.Error("Hub.Broadcast channel of wrong type")
	}
	if reflect.TypeOf(h.Register) != reflect.TypeOf(make(chan *Client)) {
		t.Error("Hub.Register channel of wrong type")
	}
	if reflect.TypeOf(h.Unregister) != reflect.TypeOf(make(chan *Client)) {
		t.Error("Hub.Unregister channel of wrong type")
	}

	if reflect.TypeOf(h.Clients) != reflect.TypeOf(make(map[string]map[*Client]bool)) {
		t.Error("Hub.Broadcast channel of wrong type")
	}

}

func TestInstantiateClient(t *testing.T) {

	h := New()
	c := &Client{Hub: h, Name: "aa", Topic: "/video0", Send: make(chan Message), Stats: NewClientStats()}

	if time.Since(c.Stats.ConnectedAt) > time.Millisecond {
		t.Error("Client connectedAt time is incorrect")
	}

	if !c.Stats.Tx.Last.IsZero() {
		t.Error("Client last Tx time is not zero", c.Stats.Tx.Last)
	}

	if c.Stats.Tx.Size.Count() != 0 {
		t.Error("Client's Tx Size stats not initialised")
	}
	if c.Stats.Tx.Mps.Count() != 0 {
		t.Error("Client's Tx MPS stats not initialised")
	}

	if !c.Stats.Rx.Last.IsZero() {
		t.Error("Client last Rx time is not zero", c.Stats.Rx.Last)
	}

	if c.Stats.Rx.Size.Count() != 0 {
		t.Error("Client's Rx Size stats not initialised")
	}
	if c.Stats.Rx.Mps.Count() != 0 {
		t.Error("Client's Rx MPS stats not initialised")
	}

}

func TestRegisterClient(t *testing.T) {

	topic := "/video0"
	h := New()
	closed := make(chan struct{})
	go h.Run(closed)
	c := &Client{Hub: h, Name: "aa", Topic: topic, Send: make(chan Message), Stats: NewClientStats()}

	h.Register <- c

	if val, ok := h.Clients[topic][c]; !ok {
		t.Error("Client not registered in topic")
	} else if val == false {
		t.Error("Client registered but not made true in map")
	}
	close(closed)
}

func TestUnRegisterClient(t *testing.T) {

	topic := "/video0"
	h := New()
	closed := make(chan struct{})
	go h.Run(closed)
	c := &Client{Hub: h, Name: "aa", Topic: topic, Send: make(chan Message), Stats: NewClientStats()}

	h.Register <- c

	if val, ok := h.Clients[topic][c]; !ok {
		t.Error("Client not registered in topic")
	} else if val == false {
		t.Error("Client registered but not made true in map")
	}

	time.Sleep(time.Millisecond)
	h.Unregister <- c
	time.Sleep(time.Millisecond)
	if val, ok := h.Clients[topic][c]; ok {
		if val {
			t.Error("Client still registered")
		}
	}
	close(closed)
}

func TestSendMessage(t *testing.T) {

	h := New()
	closed := make(chan struct{})
	go h.Run(closed)

	topicA := "/videoA"
	c1 := &Client{Hub: h, Name: "1", Topic: topicA, Send: make(chan Message), Stats: NewClientStats()}
	c2 := &Client{Hub: h, Name: "2", Topic: topicA, Send: make(chan Message), Stats: NewClientStats()}

	topicB := "/videoB"
	c3 := &Client{Hub: h, Name: "2", Topic: topicB, Send: make(chan Message), Stats: NewClientStats()}

	h.Register <- c1
	h.Register <- c2
	h.Register <- c3

	content := []byte{'t', 'e', 's', 't'}

	m := &Message{Data: content, Sender: *c1, Sent: time.Now(), Type: 0}

	var start time.Time

	rxCount := 0

	go func() {
		timer := time.NewTimer(5 * time.Millisecond)
	COLLECT:
		for {
			select {
			case <-c1.Send:
				t.Error("Sender received echo")
			case msg := <-c2.Send:
				elapsed := time.Since(start)
				if elapsed > (time.Millisecond) {
					t.Error("Message took longer than 1 millisecond, ", elapsed)
				}
				rxCount++
				if bytes.Compare(msg.Data, content) != 0 {
					t.Error("Wrong data in message")
				}
			case <-c3.Send:
				t.Error("Wrong client received message")
			case <-timer.C:
				break COLLECT
			}
		}
	}()

	time.Sleep(time.Millisecond)
	start = time.Now()
	h.Broadcast <- *m
	time.Sleep(time.Millisecond)
	if rxCount != 1 {
		t.Error("Receiver did not receive message in correct quantity, wanted 1 got ", rxCount)
	}
	close(closed)
}

func TestSendLargeMessage(t *testing.T) {

	h := New()
	closed := make(chan struct{})
	go h.Run(closed)

	topicA := "/videoA"
	c1 := &Client{Hub: h, Name: "1", Topic: topicA, Send: make(chan Message), Stats: NewClientStats()}
	c2 := &Client{Hub: h, Name: "2", Topic: topicA, Send: make(chan Message), Stats: NewClientStats()}

	topicB := "/videoB"
	c3 := &Client{Hub: h, Name: "2", Topic: topicB, Send: make(chan Message), Stats: NewClientStats()}

	h.Register <- c1
	h.Register <- c2
	h.Register <- c3

	content := make([]byte, 1024)

	rand.Read(content)

	m := &Message{Data: content, Sender: *c1, Sent: time.Now(), Type: 0}

	var start time.Time

	go func() {
		start = time.Now()
		h.Broadcast <- *m
		time.Sleep(time.Second)
	}()

	timer := time.NewTimer(time.Millisecond)

	rxCount := 0

COLLECT:
	for {
		select {
		case <-c1.Send:
			t.Error("Sender received echo")
		case msg := <-c2.Send:
			elapsed := time.Since(start)
			if elapsed > (time.Millisecond) {
				t.Error("Message took longer than 1 millisecond, size was ", len(msg.Data))
			}
			rxCount++
			if bytes.Compare(msg.Data, content) != 0 {
				t.Error("Wrong data in message")
			}
		case <-c3.Send:
			t.Error("Wrong client received message")
		case <-timer.C:
			break COLLECT
		}
	}

	if rxCount != 1 {
		t.Error("Receiver did not receive message in correct quantity, wanted 1 got ", rxCount)
	}
	close(closed)
}

func TestSendManyMessages(t *testing.T) {

	h := New()
	closed := make(chan struct{})
	go h.Run(closed)

	topicA := "/videoA"

	ca1 := &Client{Hub: h, Name: "a1", Topic: topicA, Send: make(chan Message), Stats: NewClientStats()}
	ca2 := &Client{Hub: h, Name: "a2", Topic: topicA, Send: make(chan Message), Stats: NewClientStats()}
	ca3 := &Client{Hub: h, Name: "a3", Topic: topicA, Send: make(chan Message), Stats: NewClientStats()}

	topicB := "/videoB"

	cb1 := &Client{Hub: h, Name: "b1", Topic: topicB, Send: make(chan Message), Stats: NewClientStats()}
	cb2 := &Client{Hub: h, Name: "b2", Topic: topicB, Send: make(chan Message), Stats: NewClientStats()}
	cb3 := &Client{Hub: h, Name: "b3", Topic: topicB, Send: make(chan Message), Stats: NewClientStats()}

	topicC := "/videoC"

	cc1 := &Client{Hub: h, Name: "c1", Topic: topicC, Send: make(chan Message), Stats: NewClientStats()}
	cc2 := &Client{Hub: h, Name: "c2", Topic: topicC, Send: make(chan Message), Stats: NewClientStats()}
	cc3 := &Client{Hub: h, Name: "c3", Topic: topicC, Send: make(chan Message), Stats: NewClientStats()}

	topicD := "/videoD"

	cd1 := &Client{Hub: h, Name: "d1", Topic: topicD, Send: make(chan Message), Stats: NewClientStats()}
	cd2 := &Client{Hub: h, Name: "d2", Topic: topicD, Send: make(chan Message), Stats: NewClientStats()}
	cd3 := &Client{Hub: h, Name: "d3", Topic: topicD, Send: make(chan Message), Stats: NewClientStats()}

	h.Register <- ca1
	h.Register <- ca2
	h.Register <- ca3
	h.Register <- cb1
	h.Register <- cb2
	h.Register <- cb3
	h.Register <- cc1
	h.Register <- cc2
	h.Register <- cc3
	h.Register <- cd1
	h.Register <- cd2
	h.Register <- cd3

	time.Sleep(time.Millisecond)

	if len(h.Clients[topicA]) != 3 {
		t.Error("Wrong number of clients registered for TopicA, wanted 3 got", len(h.Clients[topicA]))
	}
	if len(h.Clients[topicB]) != 3 {
		t.Error("Wrong number of clients registered for TopicB, wanted 3 got", len(h.Clients[topicB]))
	}
	if len(h.Clients[topicC]) != 3 {
		t.Error("Wrong number of clients registered for TopicC, wanted 3 got", len(h.Clients[topicB]))
	}
	if len(h.Clients[topicD]) != 3 {
		t.Error("Wrong number of clients registered for TopicD, wanted 3 got", len(h.Clients[topicB]))
	}
	contentA := make([]byte, 1024*1024*10)
	contentB := make([]byte, 1024*1024*10)
	contentC := make([]byte, 1024*1024*10)
	contentD := make([]byte, 1024*1024*10)

	rand.Read(contentA)
	rand.Read(contentB)
	rand.Read(contentC)
	rand.Read(contentD)

	mA := &Message{Data: contentA, Sender: *ca1, Sent: time.Now(), Type: 0}
	mB := &Message{Data: contentB, Sender: *cb1, Sent: time.Now(), Type: 0}
	mC := &Message{Data: contentC, Sender: *cc1, Sent: time.Now(), Type: 0}
	mD := &Message{Data: contentD, Sender: *cd1, Sent: time.Now(), Type: 0}

	rxCount := 0

	iterations := 100

	duration := time.Duration(iterations) * 6 * time.Millisecond

	go receive(&rxCount, ca2, contentA, duration, t)
	go receive(&rxCount, cb2, contentB, duration, t)
	go receive(&rxCount, cc2, contentC, duration, t)
	go receive(&rxCount, cd2, contentD, duration, t)
	go receive(&rxCount, ca3, contentA, duration, t)
	go receive(&rxCount, cb3, contentB, duration, t)
	go receive(&rxCount, cc3, contentC, duration, t)
	go receive(&rxCount, cd3, contentD, duration, t)

	time.Sleep(1 * time.Millisecond)

	for i := 0; i < iterations; i++ {
		h.Broadcast <- *mA
		h.Broadcast <- *mB
		h.Broadcast <- *mC
		h.Broadcast <- *mD
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(time.Duration(iterations+1) * 6 * time.Millisecond)

	if rxCount != iterations*8 {
		t.Error("Got wrong message count, wanted/got", iterations*8, rxCount)
	}

	close(closed)

}

func receive(counter *int, client *Client, content []byte, duration time.Duration, t *testing.T) {

	timer := time.NewTimer(duration)

COLLECT:
	for {
		select {
		case msg := <-client.Send:
			if bytes.Compare(msg.Data, content) != 0 {
				t.Error("Wrong data in message", len(msg.Data), len(content))
			} else {
				*counter++
			}
		case <-timer.C:
			break COLLECT
		}
	}
}
