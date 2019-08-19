// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

// Split takes some packet and issues the callback with the sequence of frames that
// should be sent.
func Split(pkt Packet, cb func(fr Frame) error) error {
	fr := Frame{
		Header: Header{
			PacketID: pkt.PacketID,
			FrameInfo: FrameInfo{
				Continuation: false,
				Starting:     true,
				PayloadKind:  pkt.PayloadKind,
			},
		},
	}

	for len(pkt.Data) > 1023 {
		fr.Header.Length = 1023
		fr.Header.Continuation = true
		fr.Data = pkt.Data[:1023]

		if err := cb(fr); err != nil {
			return err
		}

		fr.Header.Starting = false
		pkt.Data = pkt.Data[1023:]
	}

	if len(pkt.Data) > 0 || fr.Header.Starting {
		fr.Header.Length = uint16(len(pkt.Data))
		fr.Header.Continuation = false
		fr.Data = pkt.Data
		return cb(fr)
	}
	return nil
}
