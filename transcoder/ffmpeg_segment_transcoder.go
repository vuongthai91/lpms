package transcoder

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/livepeer/lpms/ffmpeg"
)

//SegmentTranscoder transcodes segments individually.  This is a simple wrapper for calling FFMpeg on the command line.
type FFMpegSegmentTranscoder struct {
	tProfiles []ffmpeg.VideoProfile
	workDir   string
}

func NewFFMpegSegmentTranscoder(ps []ffmpeg.VideoProfile, workd string) *FFMpegSegmentTranscoder {
	return &FFMpegSegmentTranscoder{tProfiles: ps, workDir: workd}
}

func (t *FFMpegSegmentTranscoder) Transcode(fname string) ([]string, error) {
	//Invoke ffmpeg
	err := ffmpeg.Transcode(fname, t.workDir, t.tProfiles)
	if err != nil {
		glog.Errorf("Error transcoding: %v", err)
		return nil, err
	}

	dout := make([]string, len(t.tProfiles), len(t.tProfiles))
	for i, _ := range t.tProfiles {
		dout[i] = path.Join(t.workDir, fmt.Sprintf("out%v%v", i, filepath.Base(fname)))
	}

	return dout, nil
}
