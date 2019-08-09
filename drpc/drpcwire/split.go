// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

// Split takes some data and issues the callback with the sequence of packets that
// should be sent.
func Split(kind PayloadKind, pid PacketID, data []byte, cb func(pkt Packet) error) error {
	pkt := Packet{
		Header: Header{
			FrameInfo: FrameInfo{
				Continuation: false,
				Starting:     true,
				PayloadKind:  kind,
			},
			PacketID: pid,
		},
	}

	for len(data) > 1023 {
		pkt.Header.Length = 1023
		pkt.Header.Continuation = true
		pkt.Data = data[:1023]

		if err := cb(pkt); err != nil {
			return err
		}

		pkt.Header.Starting = false
		data = data[1023:]
	}

	if len(data) > 0 || pkt.Header.Starting {
		pkt.Header.Length = uint16(len(data))
		pkt.Header.Continuation = false
		pkt.Data = data
		return cb(pkt)
	}
	return nil
}
