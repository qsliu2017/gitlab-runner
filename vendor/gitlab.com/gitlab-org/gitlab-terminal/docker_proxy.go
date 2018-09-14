package terminal

import (
	"errors"
	"fmt"
	"io"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type DockerProxy struct {
	StopCh chan error
}

// stoppers is the number of goroutines that may attempt to call Stop()
func NewDockerProxy(stoppers int) *DockerProxy {
	return &DockerProxy{
		StopCh: make(chan error, stoppers+2), // each proxy() call is a stopper
	}
}

func (p *DockerProxy) GetStopCh() chan error {
	return p.StopCh
}

func (p *DockerProxy) Serve(client Connection, docker io.ReadWriteCloser, logger *logrus.Entry) error {
	go p.read(client, docker, logger)
	go p.write(client, docker, logger)

	err := <-p.StopCh
	return err
}

func (p *DockerProxy) read(client Connection, docker io.ReadWriteCloser, logger *logrus.Entry) {
	for {
		b := make([]byte, 100)

		n, err := docker.Read(b)
		if err != nil {
			p.StopCh <- fmt.Errorf("failed to read from docker exec container: %v", err)
		}

		// No need to write to socket if we didn't read anything
		if n == 0 {
			continue
		}

		err = client.WriteMessage(websocket.BinaryMessage, b)
		if err != nil {
			p.StopCh <- fmt.Errorf("failed to write to webscoket: %v", err)
			break
		}
	}
}

func (p *DockerProxy) write(client Connection, docker io.ReadWriteCloser, logger *logrus.Entry) {
	for {
		typ, b, err := client.ReadMessage()
		if err != nil {
			docker.Close()
			p.StopCh <- fmt.Errorf("failed to read message from websocket: %v", err)
		}

		if typ != websocket.BinaryMessage {
			docker.Close()
			p.StopCh <- errors.New("message read from websocket is not binary type")
		}

		n, err := docker.Write(b)
		if err != nil {
			p.StopCh <- errors.New("failed to write to docker container")
		}

		if len(b) != n {
			logger.WithFields(logrus.Fields{
				"written":       n,
				"expectedWrite": len(b),
			}).Warn("not all bytes are written to docker container")
		}
	}
}
