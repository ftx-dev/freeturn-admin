// Package link правит share-ссылки freeturn:// (base64url(JSON), формат ядра docs/uri.md).
package link

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

const scheme = "freeturn://"

var (
	ErrBadLink     = errors.New("это не ссылка freeturn://")
	ErrBadCallLink = errors.New("ссылка на звонок должна начинаться с https://")
)

type field struct {
	key string
	raw json.RawMessage
}

// WithCallLink вкладывает ссылку на звонок (ключ "vk", как у Android и десктопных клиентов) или убирает её,
// если call пустой. Порядок остальных полей не меняется; "vk" встаёт после "name" и перед "wg" -
// там же, где его ставят клиенты, так что ссылки совпадают байт в байт.
func WithCallLink(link, call string) (string, error) {
	call = strings.TrimSpace(call)
	if call != "" {
		u, err := url.Parse(call)
		if err != nil || u.Scheme != "https" || u.Host == "" || len(call) > 2048 {
			return "", ErrBadCallLink
		}
	}
	fields, err := decode(link)
	if err != nil {
		return "", err
	}
	out := make([]field, 0, len(fields)+1)
	for _, f := range fields {
		if f.key != "vk" {
			out = append(out, f)
		}
	}
	if call != "" {
		raw, _ := marshal(call)
		at := len(out)
		for i, f := range out {
			if f.key == "wg" {
				at = i
				break
			}
		}
		out = append(out[:at], append([]field{{"vk", raw}}, out[at:]...)...)
	}
	return encode(out), nil
}

func decode(link string) ([]field, error) {
	link = strings.TrimSpace(link)
	if !strings.HasPrefix(strings.ToLower(link), scheme) {
		return nil, ErrBadLink
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(link[len(scheme):], "="))
	if err != nil {
		return nil, ErrBadLink
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, ErrBadLink
	}
	var fields []field
	for dec.More() {
		t, err := dec.Token()
		key, ok := t.(string)
		if err != nil || !ok {
			return nil, ErrBadLink
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, ErrBadLink
		}
		fields = append(fields, field{key, raw})
	}
	return fields, nil
}

// marshal - строка JSON без экранирования & < > (как в Go json.Marshal с SetEscapeHTML(false) и у клиентов).
func marshal(s string) (json.RawMessage, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

func encode(fields []field) string {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := marshal(f.key)
		b.Write(k)
		b.WriteByte(':')
		b.Write(f.raw)
	}
	b.WriteByte('}')
	return scheme + base64.RawURLEncoding.EncodeToString(b.Bytes())
}
