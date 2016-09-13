package lnwire

import "github.com/BitfuryLightning/tools/rt/graph"

type NetworkMessageBase struct {
	SenderID graph.ID
	ReceiverID graph.ID
}

func (msg *NetworkMessageBase) GetSenderID() graph.ID {
	return msg.SenderID
}

func (msg *NetworkMessageBase) GetReceiverID() graph.ID {
	return msg.ReceiverID
}

func (msg *NetworkMessageBase) SetSenderID(senderID graph.ID) {
	msg.SenderID = senderID
}

func (msg *NetworkMessageBase) SetReceiverID(receiverID graph.ID) {
	msg.ReceiverID = receiverID
}

type NetworkMessage interface {
	GetSenderID() graph.ID
	GetReceiverID() graph.ID
	SetSenderID(graph.ID)
	SetReceiverID(graph.ID)
}