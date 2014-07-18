package ts

import . "github.com/32bitkid/mpeg"

func alwaysTrue(p *Packet) bool { return true }

type PacketChannel <-chan *Packet

func (input PacketChannel) PayloadOnly() <-chan []byte {
	output := make(chan []byte)
	go func() {
		defer close(output)
		for packet := range input {
			output <- packet.Payload
		}
	}()
	return output
}

func NewDemuxer(reader BitReader) Demuxer {
	return &tsDemuxer{
		reader:    reader,
		skipUntil: alwaysTrue,
		takeWhile: alwaysTrue,
	}
}

type Demuxer interface {
	Where(PacketTester) PacketChannel
	Go() <-chan bool
	Err() error

	SkipUntil(PacketTester)
	TakeWhile(PacketTester)
}

type conditionalChannel struct {
	test    PacketTester
	channel chan<- *Packet
}

type tsDemuxer struct {
	reader             BitReader
	registeredChannels []conditionalChannel
	lastErr            error
	skipUntil          PacketTester
	takeWhile          PacketTester
}

func (tsd *tsDemuxer) Where(test PacketTester) PacketChannel {
	channel := make(chan *Packet)
	tsd.registeredChannels = append(tsd.registeredChannels, conditionalChannel{test, channel})
	return channel
}

func (tsd *tsDemuxer) SkipUntil(skipUntil PacketTester) {
	tsd.skipUntil = skipUntil
}

func (tsd *tsDemuxer) TakeWhile(takeWhile PacketTester) {
	tsd.takeWhile = takeWhile
}

func (tsd *tsDemuxer) Go() <-chan bool {

	done := make(chan bool, 1)
	var skipping = true
	var skipUntil = tsd.skipUntil
	var takeWhile = tsd.takeWhile

	go func() {

		defer func() { done <- true }()
		for _, item := range tsd.registeredChannels {
			defer close(item.channel)
		}

		for true {
			p, err := ReadPacket(tsd.reader)

			if err != nil {
				tsd.lastErr = err
				done <- true
				return
			}

			if skipping {
				if !skipUntil(p) {
					continue
				} else {
					skipping = false
				}
			} else {
				if !takeWhile(p) {
					return
				}
			}

			for _, item := range tsd.registeredChannels {
				if item.test(p) {
					item.channel <- p
				}
			}
		}
	}()

	return done
}

func (tsd *tsDemuxer) Err() error {
	return tsd.lastErr
}
