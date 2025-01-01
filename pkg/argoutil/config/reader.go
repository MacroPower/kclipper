package config

import (
	"io"
	"net/http"
)

// ReadRemoteFile issues a GET request to retrieve the contents of the specified URL as a byte array.
// The caller is responsible for checking error return values.
func ReadRemoteFile(url string) ([]byte, error) {
	var data []byte
	resp, err := http.Get(url)
	if err == nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		data, err = io.ReadAll(resp.Body)
	}
	return data, err
}
