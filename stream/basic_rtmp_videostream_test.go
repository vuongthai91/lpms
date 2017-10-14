package stream

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
)

//Testing WriteRTMP errors
var ErrPacketRead = errors.New("packet read error")
var ErrStreams = errors.New("streams error")

type BadStreamsDemuxer struct{}

func (d BadStreamsDemuxer) Close() error                     { return nil }
func (d BadStreamsDemuxer) Streams() ([]av.CodecData, error) { return nil, ErrStreams }
func (d BadStreamsDemuxer) ReadPacket() (av.Packet, error)   { return av.Packet{Data: []byte{0, 0}}, nil }

type BadPacketsDemuxer struct{}

func (d BadPacketsDemuxer) Close() error                     { return nil }
func (d BadPacketsDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d BadPacketsDemuxer) ReadPacket() (av.Packet, error) {
	return av.Packet{Data: []byte{0, 0}}, ErrPacketRead
}

type NoEOFDemuxer struct {
	c *Counter
}

type Counter struct {
	Count int
}

func (d NoEOFDemuxer) Close() error                     { return nil }
func (d NoEOFDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d NoEOFDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{}, nil
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0}}, nil
}

func TestWriteBasicRTMPErrors(t *testing.T) {
	stream := NewBasicRTMPVideoStream("test")
	err := stream.WriteRTMPToStream(context.Background(), BadStreamsDemuxer{})
	if err != ErrStreams {
		t.Error("Expecting Streams Error, but got: ", err)
	}

	err = stream.WriteRTMPToStream(context.Background(), BadPacketsDemuxer{})
	if err != ErrPacketRead {
		t.Error("Expecting Packet Read Error, but got: ", err)
	}

	err = stream.WriteRTMPToStream(context.Background(), NoEOFDemuxer{c: &Counter{Count: 0}})
	if err != ErrDroppedRTMPStream {
		t.Error("Expecting RTMP Dropped Error, but got: ", err)
	}
}

//Testing WriteRTMP
type PacketsDemuxer struct {
	c *Counter
}

func (d PacketsDemuxer) Close() error { return nil }
func (d PacketsDemuxer) Streams() ([]av.CodecData, error) {
	return []av.CodecData{h264parser.CodecData{}}, nil
}
func (d PacketsDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{Data: []byte{0, 0}}, io.EOF
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0, 0}}, nil
}

type TestMuxer struct {
	c *Counter
}

func (m TestMuxer) WriteHeader([]av.CodecData) error { m.c.Count++; return nil }
func (m TestMuxer) WritePacket(av.Packet) error      { m.c.Count++; return nil }
func (m TestMuxer) WriteTrailer() error              { m.c.Count++; return nil }
func (m TestMuxer) Close() error                     { return nil }
func TestWriteBasicRTMP(t *testing.T) {
	// stream := Stream{Buffer: NewStreamBuffer(), StreamID: "test"}
	stream := NewBasicRTMPVideoStream("test")
	tdst := dst{mux: TestMuxer{c: &Counter{Count: 0}}}
	stream.dsts = append(stream.dsts, tdst)
	err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})

	if err != io.EOF {
		t.Error("Expecting EOF, but got: ", err)
	}

	common.WaitUntil(time.Second, func() bool {
		return len(stream.header) > 0
	})
	if len(stream.header) == 0 {
		t.Errorf("Expecting header to be set")
	}

	common.WaitUntil(time.Second, func() bool {
		return tdst.mux.(TestMuxer).c.Count == 11
	})
	if tdst.mux.(TestMuxer).c.Count != 11 { //10 packets, 1 trailer
		t.Error("Expecting buffer length to be 11, but got: ", tdst.mux.(TestMuxer).c.Count)
	}

	//TODO: Test what happens when the buffer is full (should evict everything before the last keyframe)
}

var ErrBadHeader = errors.New("BadHeader")
var ErrBadPacket = errors.New("BadPacket")

type BadHeaderMuxer struct{}

func (d BadHeaderMuxer) Close() error                     { return nil }
func (d BadHeaderMuxer) WriteHeader([]av.CodecData) error { return ErrBadHeader }
func (d BadHeaderMuxer) WriteTrailer() error              { return nil }
func (d BadHeaderMuxer) WritePacket(av.Packet) error      { return nil }

type BadPacketMuxer struct{}

func (d BadPacketMuxer) Close() error                     { return nil }
func (d BadPacketMuxer) WriteHeader([]av.CodecData) error { return nil }
func (d BadPacketMuxer) WriteTrailer() error              { return nil }
func (d BadPacketMuxer) WritePacket(av.Packet) error      { return ErrBadPacket }

type ConstantDemuxer struct {
	c *Counter
}

func (d ConstantDemuxer) Close() error { return nil }
func (d ConstantDemuxer) Streams() ([]av.CodecData, error) {
	return []av.CodecData{h264parser.CodecData{}}, nil
}
func (d ConstantDemuxer) ReadPacket() (av.Packet, error) {
	time.Sleep(time.Millisecond * 100)
	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0, 0}}, nil
}

func TestReadBasicRTMPError(t *testing.T) {
	stream := NewBasicRTMPVideoStream("test")
	go func() {
		stream.WriteRTMPToStream(context.Background(), ConstantDemuxer{c: &Counter{Count: 0}})
	}()

	if err := stream.ReadRTMPFromStream(context.Background(), BadHeaderMuxer{}); err != ErrBadHeader {
		t.Error("Expecting bad header error, but got ", err)
	}

	if err := stream.ReadRTMPFromStream(context.Background(), BadPacketMuxer{}); err != ErrBadPacket {
		t.Error("Expecting bad packet error, but got ", err)
	}
}

//Test ReadRTMP
type PacketsMuxer struct{}

func (d PacketsMuxer) Close() error                     { return nil }
func (d PacketsMuxer) WriteHeader([]av.CodecData) error { return nil }
func (d PacketsMuxer) WriteTrailer() error              { return nil }
func (d PacketsMuxer) WritePacket(av.Packet) error      { return nil }

func TestReadBasicRTMP(t *testing.T) {
	// stream := NewBasicRTMPVideoStream("test")
	// err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})
	// if err != io.EOF {
	// 	t.Error("Error setting up the test - while inserting packet.")
	// }
	// readErr := stream.ReadRTMPFromStream(context.Background(), PacketsMuxer{})

	// if readErr != io.EOF {
	// 	t.Error("Expecting buffer to be empty, but got ", err)
	// }

	// if stream.buffer.len() != 0 {
	// 	t.Error("Expecting buffer length to be 0, but got ", stream.buffer.len())
	// }

}
