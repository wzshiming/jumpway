package config

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wzshiming/httpcache"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (n *NoProxy) GetList(configDir string) []string {
	set := map[string]struct{}{}
	for _, item := range n.List {
		setEnv(set, item)
	}
	for _, env := range n.FromEnv {
		setEnv(set, os.Getenv(env))
	}
	for _, file := range n.FromFile {
		f, err := getFile(configDir, file)
		if err != nil {
			log.Error(err, i18n.OpenFile(), "file", file)
			continue
		}
		reader := bufio.NewReader(f)
		for i := 0; ; i++ {
			line, _, err := reader.ReadLine()
			if err != nil {
				if err != io.EOF {
					log.Error(err, i18n.OpenFile(), "file", file, "line", i+1)
				}
				break
			}
			setEnv(set, string(line))
		}
		f.Close()
	}
	list := make([]string, 0, len(set))
	for item := range set {
		list = append(list, item)
	}
	sort.Strings(list)
	return list
}

func setEnv(set map[string]struct{}, val string) {
	val = strings.TrimSpace(val)
	if val == "" {
		return
	}
	for _, m := range strings.Split(val, ",") {
		m = strings.TrimSpace(m)
		set[m] = struct{}{}
	}
}

func getFile(configDir, filePath string) (io.ReadCloser, error) {
	u, err := url.Parse(filePath)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "http", "https":
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		client := *httpCli
		client.Transport = httpcache.NewRoundTripper(client.Transport, httpcache.WithStorer(
			httpcache.DirectoryStorer(filepath.Join(configDir, "cache")),
		))
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status %s for %s", resp.Status, u.String())
		}
		return resp.Body, nil
	case "file", "":
		file := u.Path
		if strings.HasPrefix(file, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			file = filepath.Join(home, file[1:])
		} else if strings.HasPrefix(file, ".") {
			file = filepath.Join(configDir, file[1:])
		}
		body, err := os.OpenFile(file, os.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		return body, nil
	default:
		return nil, fmt.Errorf("unknown scheme %v", u.Scheme)
	}
}

var httpCli = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}
