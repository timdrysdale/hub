package hub

import (
	"time"

	"github.com/eclesh/welford"
)

func New() *Hub {
	return &Hub{
		Broadcast:  make(chan Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[string]map[*Client]bool),
	}
}

func NewClientStats() *ClientStats {

	c := &ClientStats{}
	c.ConnectedAt = time.Now()

	c.Rx = &Frames{Size: welford.New(), Mps: welford.New()}
	c.Tx = &Frames{Size: welford.New(), Mps: welford.New()}
	return c
}

func (h *Hub) Run(closed chan struct{}) {
	for {
		select {
		case <-closed:
			return
		case client := <-h.Register:
			if _, ok := h.Clients[client.Topic]; !ok {
				h.Clients[client.Topic] = make(map[*Client]bool)
			}
			h.Clients[client.Topic][client] = true
		case client := <-h.Unregister:
			if _, ok := h.Clients[client.Topic]; ok {
				delete(h.Clients[client.Topic], client)
				close(client.Send)
			}
		case message := <-h.Broadcast:
			topic := message.Sender.Topic
			for client := range h.Clients[topic] {
				if client.Name != message.Sender.Name {
					select {
					case client.Send <- message:
					default:
						close(client.Send)
						delete(h.Clients[topic], client)
					}
				}
			}
		}
	}
}
