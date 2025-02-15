package helmtest

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type TestPathEncoder struct{}

func (*TestPathEncoder) Encode(s string) string {
	o := &struct {
		Chart   string `json:"chart"`
		Project string `json:"project"`
		URL     string `json:"url"`
		Version string `json:"version"`
	}{}

	err := json.Unmarshal([]byte(s), o)
	if err != nil {
		panic(fmt.Errorf("failed to encode key %q: %w", s, err))
	}

	key := fmt.Sprintf("%s__%s__%s__%s", o.Chart, o.Project, o.URL, o.Version)

	return url.PathEscape(key)
}

func (*TestPathEncoder) Decode(s string) (string, error) {
	d, err := url.PathUnescape(s)
	attrs := strings.Split(d, "__")
	key := fmt.Sprintf(`{"chart":%q,"project":%q,"url":%q,"version":%q}`, attrs[0], attrs[1], attrs[2], attrs[3])

	return key, fmt.Errorf("failed to decode key %q: %w", s, err)
}
