package lnwire

import (
	"io"
	"fmt"

	"github.com/BitfuryLightning/tools/network/idh"
)

type NetworkResponseAllMessage struct {
	NetworkMessageBase
	Info *idh.LightningIDToHost
}

func (msg *NetworkResponseAllMessage) Decode(r io.Reader, pver uint32) error {
	info, err := idh.UnmarshalLightningIDToHost(r)
	msg.Info = info
	return err
} 

func (msg *NetworkResponseAllMessage) Encode(w io.Writer, pver uint32) error {
	return msg.Info.Marshal(w)
}

func (msg *NetworkResponseAllMessage) Command() uint32 {
	return CmdNetworkResponseAllMessage
}

func (msg *NetworkResponseAllMessage) MaxPayloadLength(uint32) uint32 {
	return MaxMessagePayload
}

func (msg *NetworkResponseAllMessage) Validate() error {
	return nil
}

func (msg *NetworkResponseAllMessage) String() string {
	return fmt.Sprintf("NetworkRequestAllMessage{%v %v %v}", msg.SenderID, msg.ReceiverID, msg.Info)
}