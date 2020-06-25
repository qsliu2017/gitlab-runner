package helpers

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestHealthProbeCommandExecute(t *testing.T) {
	cases := []struct {
		name            string
		expectedConnect bool
	}{
		{
			name:            "Successful connect",
			expectedConnect: true,
		},
		{
			name:            "Unsuccessful connect because service is down",
			expectedConnect: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Start listening to reverse addr
			listener, err := net.Listen("tcp", "127.0.0.1:")
			require.NoError(t, err)
			defer listener.Close()

			// If we don't expect to connect we close the listener.
			if !c.expectedConnect {
				listener.Close()
			}

			ctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelFn()
			done := make(chan struct{})
			go func() {
				app := cli.NewApp()
				app.Commands = []cli.Command{healthProbeCommand}

				_ = app.Run([]string{"command", "health-probe",
					"--host", "127.0.0.1",
					"--period", "5s",
					"--retries", "2",
					"tcp",
					"--port", strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)})

				done <- struct{}{}
			}()

			select {
			case <-ctx.Done():
				if c.expectedConnect {
					require.Fail(t, "Timeout waiting to start service.")
				}
			case <-done:
				if !c.expectedConnect {
					require.Fail(t, "Expected to not connect to server")
				}
			}
		})
	}
}
