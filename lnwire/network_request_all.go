package lnwire

import (
	"io"
	"fmt"
	"errors"
	"bytes"
)

type NetworkRequestAllMessage struct {
	NetworkMessageBase
}

func (msg *NetworkRequestAllMessage) Decode(r io.Reader, pver uint32) error {
	buff := make([]byte, 24)
	_, err := r.Read(buff)
	if err != nil {
		return err
	}
	if !bytes.Equal(buff, []byte("NetworkRequestAllMessage")) {
		return errors.New("Can't decode NetworkRequestAllMessage message")
	} else {
		fmt.Printf("SUCCESS\n")
	}
	return nil
} 

func (msg *NetworkRequestAllMessage) Encode(w io.Writer, pver uint32) error {
	w.Write([]byte("NetworkRequestAllMessage"))
	return nil
}

func (msg *NetworkRequestAllMessage) Command() uint32 {
	return CmdNetworkRequestAllMessage
}

func (msg *NetworkRequestAllMessage) MaxPayloadLength(uint32) uint32 {
	return 24
}

func (msg *NetworkRequestAllMessage) Validate() error {
	return nil
}

func (msg *NetworkRequestAllMessage) String() string {
	return fmt.Sprintf("NetworkRequestAllMessage{%v %v}", msg.SenderID, msg.ReceiverID)
}