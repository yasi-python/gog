package subscription

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	vmessRe   = regexp.MustCompile(`vmess://[A-Za-z0-9+/=]+`)
	genericRe = regexp.MustCompile(`(vless://[^\s]+|trojan://[^\s]+|ss://[^\s]+|socks5://[^\s]+)`)
)

type SourceFetcher interface {
	Fetch(ctx context.Context, url string) (string, error)
}

type HTTPFetcher struct {
	Client *http.Client
}

func (h HTTPFetcher) Fetch(ctx context.Context, url string) (string, error) {
	if h.Client == nil {
		h.Client = &http.Client{Timeout: 12 * time.Second}
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := h.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("http_status_" + resp.Status)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func TryDecodeIfBase64Block(s string) string {
	t := strings.TrimSpace(s)
	if len(t) < 60 { return s }
	candidate := strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r=='+' || r=='/' || r=='=' || r=='\n' || r=='\r' {
			return r
		}
		return -1
	}, t)
	if len(candidate) < len(t)/2 { return s }
	dec, err := base64.StdEncoding.DecodeString(candidate)
	if err != nil { return s }
	txt := string(dec)
	if strings.Contains(txt, "vmess://") || strings.Contains(txt, "vless://") || strings.Contains(txt, "trojan://") || strings.Contains(txt, "ss://") {
		return txt
	}
	return s
}

func ExtractNodes(text string) []string {
	txt := TryDecodeIfBase64Block(text)
	nodes := []string{}
	nodes = append(nodes, vmessRe.FindAllString(txt, -1)...)
	nodes = append(nodes, genericRe.FindAllString(txt, -1)...)
	for _, line := range strings.Split(txt, "\n") {
		l := strings.TrimSpace(line)
		if l == "" { continue }
		if strings.HasPrefix(strings.ToLower(l), "vmess://") ||
			strings.HasPrefix(strings.ToLower(l), "vless://") ||
			strings.HasPrefix(strings.ToLower(l), "trojan://") ||
			strings.HasPrefix(strings.ToLower(l), "ss://") ||
			strings.HasPrefix(strings.ToLower(l), "socks5://") {
			nodes = append(nodes, l)
		}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, n := range nodes {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}