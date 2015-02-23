package notifications

import (
	"encoding/json"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
	context "golang.org/x/net/context"
)

const RoutingQueryKey = "RoutingQueryEvent"

type QueryEventType int

const (
	SendingQuery QueryEventType = iota
	PeerResponse
	FinalPeer
	QueryError
	Provider
)

type QueryEvent struct {
	ID        peer.ID
	Type      QueryEventType
	Responses []*peer.PeerInfo
	Extra     string
}

func RegisterForQueryEvents(ctx context.Context, ch chan<- *QueryEvent) context.Context {
	return context.WithValue(ctx, RoutingQueryKey, ch)
}

func PublishQueryEvent(ctx context.Context, ev *QueryEvent) {
	ich := ctx.Value(RoutingQueryKey)
	if ich == nil {
		return
	}

	ch, ok := ich.(chan<- *QueryEvent)
	if !ok {
		return
	}

	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}

func (qe *QueryEvent) MarshalJSON() ([]byte, error) {
	out := make(map[string]interface{})
	out["ID"] = peer.IDB58Encode(qe.ID)
	out["Type"] = int(qe.Type)
	out["Responses"] = qe.Responses
	out["Extra"] = qe.Extra
	return json.Marshal(out)
}

func (qe *QueryEvent) UnmarshalJSON(b []byte) error {
	temp := struct {
		ID        string
		Type      int
		Responses []*peer.PeerInfo
		Extra     string
	}{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}
	if len(temp.ID) > 0 {
		pid, err := peer.IDB58Decode(temp.ID)
		if err != nil {
			return err
		}
		qe.ID = pid
	}
	qe.Type = QueryEventType(temp.Type)
	qe.Responses = temp.Responses
	qe.Extra = temp.Extra
	return nil
}
