package client

import (
	"github.com/hashicorp/vault/api"
)

type Client interface{}

func New() Client {
	return new(client)
}

type client struct {
	c *api.Client
}
