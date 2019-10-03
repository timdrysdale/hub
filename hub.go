package hub

import (
	"time"

	"github.com/eclesh/welford"
	log "github.com/sirupsen/logrus"
)

func New() *Hub {
	return &Hub{
		Broadcast:  make(chan Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[string]map[*Client]bool),
		Stats: HubStats{Audience: welford.New(),
			Bytes:   welford.New(),
			Latency: welford.New(),
			Dt:      welford.New()},
	}
}

func NewClientStats() *ClientStats {

	c := &ClientStats{}
	c.ConnectedAt = time.Now()

	c.Rx = &Frames{Size: welford.New(), Dt: welford.New()}
	c.Tx = &Frames{Size: welford.New(), Dt: welford.New()}
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
						// try again, but if it fails to send within one second, delete the client
						go func() {
							start := time.Now()
							select {
							case client.Send <- message:
								//warn about the delay as this ideally should not happen
								log.WithFields(log.Fields{"client": client, "topic": topic, "delay": time.Since(start)}).Warn("Hub Message Send Delayed")
							case <-time.After(time.Second): //TODO make timeout configurable
								log.WithFields(log.Fields{"client": client, "topic": topic}).Error("Hub Message Send Timed Out")
								//close(client.Send)
								select {
								case <-closed:
									//channel probably already closed if we have been shutdown
								default:
									close(client.Send)
								}
								delete(h.Clients[topic], client)
							case <-closed:
							}
						}()
					}
				}
			}
		}
	}
}

func (h *Hub) RunWithStats(closed chan struct{}) {
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

			// update hub statistics
			dt := time.Since(h.Stats.Last)
			if dt < 24*time.Hour {
				h.Stats.Dt.Add(float64(dt.Seconds()))
			}
			h.Stats.Last = time.Now()
			byteCount := float64(len(message.Data)) //reuse below
			h.Stats.Bytes.Add(byteCount)
			h.Stats.Audience.Add(float64(len(h.Clients[topic])))

			//distribute messages
			for client := range h.Clients[topic] {
				if client.Name != message.Sender.Name {
					select {
					case client.Send <- message:
						//update client RX statistics
						dt := time.Since(client.Stats.Rx.Last)
						if dt < 24*time.Hour {
							client.Stats.Rx.Dt.Add(float64(dt.Seconds()))
						}
						client.Stats.Rx.Last = time.Now()
						client.Stats.Rx.Size.Add(byteCount)
					default:
						go func() {
							start := time.Now()
							select {
							case client.Send <- message:
								//warn about the delay as this ideally should not happen
								log.WithFields(log.Fields{"client": client, "topic": topic, "delay": time.Since(start)}).Warn("Hub Message Send Delayed")
							case <-time.After(time.Second): //TODO make timeout configurable
								log.WithFields(log.Fields{"client": client, "topic": topic}).Error("Hub Message Send Timed Out")
								select {
								case <-closed:
									//channel probably already closed if we have been shutdown
								default:
									close(client.Send)
								}

								delete(h.Clients[topic], client)
							case <-closed:
							}
						}()
					}
				} else {
					//update client TX statistics
					dt := time.Since(client.Stats.Tx.Last)
					if dt < 24*time.Hour {
						client.Stats.Tx.Dt.Add(float64(dt.Seconds()))
					}
					client.Stats.Tx.Last = time.Now()
					client.Stats.Tx.Size.Add(byteCount)
				}
			}
			//update latency statistic for hub
			h.Stats.Latency.Add(float64(time.Since(message.Sent).Seconds()))
		}
	}
}
