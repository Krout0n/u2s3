package aggregator

import (
	"io"
	"regexp"
	"time"

	lio "github.com/taku-k/log2s3-go/pkg/io"
)

var reTsv = regexp.MustCompile(`(?:^|[ \t])time\:([^\t]+)`)

type Aggregator struct {
	reader lio.BufferedReader
	mngr   *EpochManager
	cmpr   *Compressor
	up     *Uploader
	logFmt string
	keyFmt string
	step   int
	output string
}

func NewAggregator(reader lio.BufferedReader, logFmt, keyFmt, output string, step int) *Aggregator {
	mngr := NewEpochManager()
	cmpr := NewCompressor()
	up := NewUploader()
	return &Aggregator{
		reader: reader,
		mngr:   mngr,
		cmpr:   cmpr,
		up:     up,
		logFmt: logFmt,
		keyFmt: keyFmt,
		output: output,
		step:   step,
	}
}

func (a *Aggregator) Run() error {
	defer a.Close()

	for {
		l, err := a.reader.Readln()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		epochKey := a.parseEpoch(string(l))
		if epochKey == "" {
			continue
		}
		var epoch *Epoch
		if !a.mngr.HasEpoch(epochKey) {
			epoch, err = NewEpoch(epochKey, a.keyFmt, a.output)
			if err != nil {
				return err
			}
			a.mngr.PutEpoch(epoch)
		} else {
			epoch = a.mngr.GetEpoch(epochKey)
		}
		epoch.Write(l)
	}
	for _, e := range a.mngr.epochs {
		if err := a.up.Upload(e); err != nil {
			return err
		}
	}
	return nil
}

func (a *Aggregator) Close() {
	a.reader.Close()
	a.mngr.Close()
}

func (a *Aggregator) parseEpoch(l string) string {
	r := ""
	switch a.logFmt {
	case "ssv":
		break
	case "tsv":
		m := reTsv.FindStringSubmatch(l)
		if len(m) == 2 {
			r = m[1]
		}
		break
	}
	if r == "" {
		return ""
	}
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", r)
	if err != nil {
		return ""
	}
	e := time.Unix(t.Unix()-t.Unix()%(int64(a.step)*60), 0)
	return e.Format("20060102150405")
}
