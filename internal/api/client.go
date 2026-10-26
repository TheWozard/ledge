package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Client calls the API of the ledge on one socket.
type Client struct {
	Socket string
}

// Call sends body (nil for none) and decodes the reply into out. An error reply
// becomes an error carrying the server's message.
func (c Client) Call(method, path string, body []byte, out any) error {
	// The host is a placeholder: every request goes to the socket.
	request, err := http.NewRequest(method, "http://ledge"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http().Do(request)
	if err != nil {
		return fmt.Errorf("no ledge API at %s (%w); is ledge running?", c.Socket, unwrapURL(err))
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		var reply struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &reply) == nil && reply.Error != "" {
			return errors.New(reply.Error)
		}
		return fmt.Errorf("%s %s: %s", method, path, response.Status)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(out)
}

func (c Client) http() *http.Client {
	var dialer net.Dialer
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			// A connection is cheap, and a kept one would outlive this client.
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", c.Socket)
			},
		},
	}
}

// unwrapURL drops the "Get http://...:" that net/http puts in front of the cause.
func unwrapURL(err error) error {
	if inner := errors.Unwrap(err); inner != nil {
		return inner
	}
	return err
}
