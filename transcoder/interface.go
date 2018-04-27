package transcoder

type Transcoder interface {
	Transcode(fname string) ([]string, error)
}
