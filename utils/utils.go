package utils

func WebsocketProtocolFor(httpProtocol string) string {
	if httpProtocol == "https" {
		return "wss"
	}

	return "ws"
}
