package pluginutils

import (
	"bufio"
	"io"
	"log"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/whywaita/myshoes/api/proto/plugin"
)

type GRPCStdioServer struct {
	stdoutCh <-chan []byte
	stderrCh <-chan []byte
}

func (s *GRPCStdioServer) StreamStdio(_ *empty.Empty, srv plugin.GRPCStdio_StreamStdioServer) error {
	// Share the same data value between runs. Sending this over the wire
	// marshals it so we can reuse this.
	var data plugin.StdioData

	for {
		// Read our data
		select {
		case data.Data = <-s.stdoutCh:
			data.Channel = plugin.StdioData_STDOUT

		case data.Data = <-s.stderrCh:
			data.Channel = plugin.StdioData_STDERR

		case <-srv.Context().Done():
			return nil
		}

		// Not sure if this is possible, but if we somehow got here and
		// we didn't populate any data at all, then just continue.
		if len(data.Data) == 0 {
			continue
		}

		// Send our data to the client.
		if err := srv.Send(&data); err != nil {
			return err
		}
	}
}

func NewGRPCStdioServer() (*GRPCStdioServer, error) {
	var err error
	var stdoutR, stderrR io.Reader

	stdoutR, _, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	stderrR, _, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	stdoutR = io.TeeReader(stdoutR, os.Stdout)
	stderrR = io.TeeReader(stderrR, os.Stderr)

	stdoutCh := make(chan []byte)
	stderrCh := make(chan []byte)

	go copyChan(stdoutCh, stdoutR)
	go copyChan(stderrCh, stderrR)

	return &GRPCStdioServer{
		stdoutCh: stdoutCh,
		stderrCh: stderrCh,
	}, nil
}

// copyChan copies an io.Reader into a channel.
func copyChan(dst chan<- []byte, src io.Reader) {
	bufsrc := bufio.NewReader(src)

	for {
		// Make our data buffer. We allocate a new one per loop iteration
		// so that we can send it over the channel.
		var data [1024]byte

		// Read the data, this will block until data is available
		n, err := bufsrc.Read(data[:])

		// We have to check if we have data BEFORE err != nil. The bufio
		// docs guarantee n == 0 on EOF but its better to be safe here.
		if n > 0 {
			// We have data! Send it on the channel. This will block if there
			// is no reader on the other side. We expect that go-plugin will
			// connect immediately to the stdio server to drain this so we want
			// this block to happen for backpressure.
			dst <- data[:n]
		}

		// If we hit EOF we're done copying
		if err == io.EOF {
			log.Println("stdio EOF, exiting copy loop")
			return
		}

		// Any other error we just exit the loop. We don't expect there to
		// be errors since our use case for this is reading/writing from
		// a in-process pipe (os.Pipe).
		if err != nil {
			log.Printf("error copying stdio data, stopping copy: %+v", err)
			return
		}
	}
}
