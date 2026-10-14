package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the API of the ledge on one port.
type Client struct {
	Port int
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

// Call sends body (nil for none) and decodes the reply into out. An error reply
// becomes an error carrying the server's message.
func (c Client) Call(method, path string, body []byte, out any) error {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", c.Port, path)
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("no ledge API on 127.0.0.1:%d (%w); is ledge running?", c.Port, unwrapURL(err))
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

// unwrapURL drops the "Get http://...:" that net/http puts in front of the cause.
func unwrapURL(err error) error {
	if inner := errors.Unwrap(err); inner != nil {
		return inner
	}
	return err
}
