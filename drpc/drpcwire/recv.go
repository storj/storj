// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"bufio"
	"io"
	"sync"

	"storj.io/storj/drpc"
)

type Receiver struct {
	mu       sync.Mutex
	err      error
	scanner  *bufio.Scanner
	pending  map[PacketID]Packet
	messages map[uint64]map[uint64]struct{}
	size     uint64
}

func NewReceiver(r io.Reader) *Receiver {
	return &Receiver{
		scanner:  NewScanner(r),
		pending:  make(map[PacketID]Packet),
		messages: make(map[uint64]map[uint64]struct{}),
	}
}

// FreeStreamID removes all pending data for the provided stream id.
func (r *Receiver) FreeStreamID(sid uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for mid := range r.messages[sid] {
		pid := PacketID{StreamID: sid, MessageID: mid}
		r.size -= uint64(len(r.pending[pid].Data))
		delete(r.pending, pid)
	}
	delete(r.messages, sid)
}

func (r *Receiver) recordPacket(pkt Packet) {
	mids, ok := r.messages[pkt.StreamID]
	if !ok {
		mids = make(map[uint64]struct{})
		r.messages[pkt.StreamID] = mids
	}

	mids[pkt.MessageID] = struct{}{}
	r.pending[pkt.PacketID] = pkt
}

func (r *Receiver) freePacket(pkt Packet) {
	delete(r.messages[pkt.StreamID], pkt.MessageID)
	if len(r.messages[pkt.StreamID]) == 0 {
		delete(r.messages, pkt.StreamID)
	}
	delete(r.pending, pkt.PacketID)
}

// ReadPacket reads a fully formed Packet from the underlying reader and returns it.
func (r *Receiver) ReadPacket() (pkt *Packet, err error) {
	var (
		rem []byte
		fr  Frame
	)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Save any returned error so that it's persistent.
	defer func() {
		if r.err == nil && err != nil {
			r.err = err

			// explicitly free these buffers to keep memory low
			r.pending = nil
			r.messages = nil
			r.scanner = nil
		}
	}()

	for {
		if r.err != nil {
			return nil, r.err
		}

		// Drop the lock temporarily while we scan.
		r.mu.Unlock()
		ok := r.scanner.Scan()
		r.mu.Lock()

		if !ok {
			return nil, drpc.Error.Wrap(r.scanner.Err())
		}

		// the scanner should return us exactly one packet, so if there's remaining
		// bytes or if it didn't parse, then there's some internal error.
		rem, fr, ok, err = ParseFrame(r.scanner.Bytes())
		switch {
		case err != nil:
			return nil, drpc.ProtocolError.Wrap(err)
		case !ok:
			return nil, drpc.InternalError.New("invalid parse after scanner")
		case len(rem) != 0:
			return nil, drpc.ProtocolError.New("remaining bytes from parsing packet")
		case fr.Length == 0 &&
			fr.PayloadKind != PayloadKind_Cancel &&
			fr.PayloadKind != PayloadKind_Close &&
			fr.PayloadKind != PayloadKind_CloseSend:
			return nil, drpc.ProtocolError.New("invalid zero data length packet sent")
		case len(fr.Data) != int(fr.Length):
			return nil, drpc.ProtocolError.New("invalid length of data and header length")
		case fr.Length == 0 && fr.Continuation:
			return nil, drpc.ProtocolError.New("invalid send of zero length continuation")
		}

		// get the payload state for the packet and ensure that the starting bit on the
		// frame is consistent with the payload state's existence.
		pkt, packetExists := r.pending[fr.PacketID]
		switch {
		case !packetExists && !fr.Starting:
			return nil, drpc.ProtocolError.New("unknown packet id with no starting bit")
		case packetExists && fr.Starting:
			return nil, drpc.ProtocolError.New("starting packet id that already exists")
		case packetExists && pkt.PayloadKind != fr.PayloadKind:
			return nil, drpc.ProtocolError.New("changed payload kind for in flight message")
		}

		// increment the size by the extra amount
		r.size += uint64(len(fr.Data))
		pkt = Packet{
			PacketID:    fr.PacketID,
			PayloadKind: fr.PayloadKind,
			Data:        append(pkt.Data, fr.Data...),
		}

		// if we have a complete packet. we no longer need any state about it and the
		// packet is now complete, so we clean up and return the packet
		if !fr.Continuation {
			r.size -= uint64(len(pkt.Data))
			r.freePacket(pkt)
			return &pkt, nil
		}

		// if we're over size now then that's an error
		if r.size > 10<<20 {
			return nil, drpc.ProtocolError.New("too much packet data buffered")
		}
		r.recordPacket(pkt)
	}
}
