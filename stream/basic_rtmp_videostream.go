package stream

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/nareix/joy4/av"
)

type dst struct {
	mux     av.MuxCloser
	errchan chan error
}

type BasicRTMPVideoStream struct {
	streamID string
	// buffer      *streamBuffer
	dataChan    chan av.Packet
	RTMPTimeout time.Duration
	header      []av.CodecData
	dsts        []dst
}

//NewBasicRTMPVideoStream creates a new BasicRTMPVideoStream.  The default RTMPTimeout is set to 10 milliseconds because we assume all RTMP streams are local.
func NewBasicRTMPVideoStream(id string) *BasicRTMPVideoStream {
	strm := &BasicRTMPVideoStream{
		dataChan: make(chan av.Packet),
		streamID: id,
		dsts:     make([]dst, 0, 0),
	}

	go func(strm *BasicRTMPVideoStream) {
		for {
			select {
			case data, ok := <-strm.dataChan:
				if !ok {
					for _, dst := range strm.dsts {
						if err := dst.mux.WriteTrailer(); err != nil {
							glog.Errorf("Error writing RTMP trailer from Stream %v", strm.streamID)
							dst.errchan <- err
							// return
						}
					}
					return
				}
				for i, dst := range strm.dsts {
					if err := dst.mux.WritePacket(data); err != nil {
						glog.Errorf("Error writing RTMP packet from Stream %v to mux: %v", strm.streamID, err)
						strm.dsts = append(strm.dsts[:i], strm.dsts[i+1:]...)
						dst.errchan <- err
						// return
					}
				}
			}
		}
	}(strm)

	return strm
}

func (s *BasicRTMPVideoStream) GetStreamID() string {
	return s.streamID
}

func (s *BasicRTMPVideoStream) GetStreamFormat() VideoFormat {
	return RTMP
}

//ReadRTMPFromStream reads the content from the RTMP stream out into the dst.
func (s *BasicRTMPVideoStream) ReadRTMPFromStream(ctx context.Context, dstMux av.MuxCloser) error {
	defer dstMux.Close()

	//Wait for a little bit - sometimes the header gets populated a little later.
	common.WaitUntil(time.Millisecond*300, func() bool {
		return len(s.header) != 0
	})
	if len(s.header) == 0 {
		return io.EOF
	}
	if err := dstMux.WriteHeader(s.header); err != nil {
		glog.Errorf("Error writing RTMP header from Stream %v to mux", s.streamID)
		return err
	}
	errchan := make(chan error)
	s.dsts = append(s.dsts, dst{mux: dstMux, errchan: errchan})
	select {
	case err := <-errchan:
		return err
	}
}

//WriteRTMPToStream writes a video stream from src into the stream.
func (s *BasicRTMPVideoStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error {
	defer src.Close()

	//Set header in case we want to use it.
	h, err := src.Streams()
	if err != nil {
		return err
	}
	s.header = h

	c := make(chan error, 1)
	go func() {
		c <- func() error {
			for {
				packet, err := src.ReadPacket()
				if err == io.EOF {
					close(s.dataChan)
					return err
				} else if err != nil {
					return err
				} else if len(packet.Data) == 0 { //TODO: Investigate if it's possible for packet to be nil (what happens when RTMP stopped publishing because of a dropped connection? Is it possible to have err and packet both nil?)
					return ErrDroppedRTMPStream
				}

				s.dataChan <- packet
			}
		}()
	}()

	select {
	case <-ctx.Done():
		glog.V(2).Infof("Finished writing RTMP to Stream %v", s.streamID)
		return ctx.Err()
	case err := <-c:
		return err
	}
}

func (s BasicRTMPVideoStream) String() string {
	return fmt.Sprintf("StreamID: %v, Type: %v", s.GetStreamID(), s.GetStreamFormat())
}

func (s BasicRTMPVideoStream) Height() int {
	for _, cd := range s.header {
		if cd.Type().IsVideo() {
			return cd.(av.VideoCodecData).Height()
		}
	}

	return 0
}

func (s BasicRTMPVideoStream) Width() int {
	for _, cd := range s.header {
		if cd.Type().IsVideo() {
			return cd.(av.VideoCodecData).Width()
		}
	}

	return 0
}
