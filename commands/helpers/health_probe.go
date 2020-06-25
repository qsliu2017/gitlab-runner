package helpers

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/probe"
)

// ErrHealthProbeFailed is returned when a healthprobe fails.
var ErrHealthProbeFailed = errors.New("healthprobe failed")

var healthProbeCommand = cli.Command{
	Name:  "health-probe",
	Usage: "health probe utilities",
	Flags: []cli.Flag{
		cli.IntFlag{Name: "retries", Value: 1, Usage: "number of retries"},
		cli.DurationFlag{Name: "initial-delay", Value: time.Second, Usage: "initial delay"},
		cli.DurationFlag{Name: "period", Value: time.Second, Usage: "period"},
		cli.DurationFlag{Name: "timeout", Value: time.Second, Usage: "timeout"},
		cli.StringFlag{Name: "host", Value: "localhost", Usage: "host"},
	},
	Subcommands: []cli.Command{
		{
			Name:  "tcp",
			Usage: "tcp probe",
			Flags: []cli.Flag{
				cli.IntFlag{Name: "port", Value: 80, Usage: "port"},
			},
			Action: func(c *cli.Context) error {
				p := c.Parent()
				success := probe.Probe(
					context.Background(),
					logrus.StandardLogger(),
					&probe.TCPProbe{
						Host: p.String("host"),
						Port: p.String("port"),
					},
					probe.Config{
						Retries:      p.Int("retries"),
						InitialDelay: p.Duration("initial-delay"),
						Period:       p.Duration("period"),
						Timeout:      p.Duration("timeout"),
					})

				if !success {
					return ErrHealthProbeFailed
				}
				return nil
			},
		},
		{
			Name:  "http-get",
			Usage: "http get probe",
			Flags: []cli.Flag{
				cli.IntFlag{Name: "port", Value: 80, Usage: "port"},
				cli.StringFlag{Name: "scheme", Value: "http", Usage: "http scheme"},
				cli.StringFlag{Name: "path", Value: "/", Usage: "http path"},
				cli.StringSliceFlag{Name: "header", Usage: "http header"},
			},
			Action: func(c *cli.Context) error {
				p := c.Parent()
				success := probe.Probe(
					context.Background(),
					logrus.StandardLogger(),
					&probe.HTTPGetProbe{
						Host:    p.String("host"),
						Port:    p.String("port"),
						Scheme:  c.String("scheme"),
						Path:    c.String("path"),
						Headers: c.StringSlice("header"),
					},
					probe.Config{
						Retries:      p.Int("retries"),
						InitialDelay: p.Duration("initial-delay"),
						Period:       p.Duration("period"),
						Timeout:      p.Duration("timeout"),
					})

				if !success {
					return ErrHealthProbeFailed
				}
				return nil
			},
		},
	},
}

func init() {
	common.RegisterCommand(healthProbeCommand)
}
