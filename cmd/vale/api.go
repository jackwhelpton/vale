package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/jdx/go-netrc"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/pflag"

	"github.com/errata-ai/vale/v3/internal/core"
)

// Style represents an externally-hosted style.
type Style struct {
	// User-provided fields.
	Author      string `json:"author"`
	Description string `json:"description"`
	Deps        string `json:"deps"`
	Feed        string `json:"feed"`
	Homepage    string `json:"homepage"`
	Name        string `json:"name"`
	URL         string `json:"url"`

	// Generated fields.
	HasUpdate bool `json:"has_update"`
	InLibrary bool `json:"in_library"`
	Installed bool `json:"installed"`
	Addon     bool `json:"addon"`
}

// Meta represents an installed style's meta data.
type Meta struct {
	Author      string   `json:"author"`
	Coverage    float64  `json:"coverage"`
	Description string   `json:"description"`
	Email       string   `json:"email"`
	Feed        string   `json:"feed"`
	Lang        string   `json:"lang"`
	License     string   `json:"license"`
	Name        string   `json:"name"`
	Sources     []string `json:"sources"`
	URL         string   `json:"url"`
	Vale        string   `json:"vale_version"`
	Version     string   `json:"version"`
}

func init() {
	pflag.BoolVar(&Flags.Remote, "mode-rev-compat", false,
		"prioritize local Vale configurations")
	pflag.StringVar(&Flags.Built, "built", "", "post-processed file path")

	Actions["install"] = install
}

func fetch(src, dst string) error {
	// Fetch the resource from the web:
	resp, err := httpGet(src)

	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not fetch '%s' (status code '%d')", src, resp.StatusCode)
	}

	// Create a temp file to represent the archive locally:
	tmpfile, err := os.CreateTemp("", "temp.*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// Write to the  local archive:
	_, err = io.Copy(tmpfile, resp.Body)
	if err != nil {
		return err
	} else if err = tmpfile.Close(); err != nil {
		return err
	}

	resp.Body.Close()
	return archiver.Unarchive(tmpfile.Name(), dst)
}

func install(args []string, flags *core.CLIFlags) error {
	cfg, err := core.ReadPipeline(flags, false)
	if err != nil {
		return err
	}

	style := filepath.Join(cfg.StylesPath(), args[0])
	if core.IsDir(style) {
		os.RemoveAll(style) // Remove existing version
	}

	err = fetch(args[1], cfg.StylesPath())
	if err != nil {
		return sendResponse(
			fmt.Sprintf("Failed to install '%s'", args[1]),
			err)
	}

	return sendResponse(fmt.Sprintf(
		"Successfully installed '%s'", args[1]), nil)
}

func httpGet(src string) (*http.Response, error) {
	f := netrcPath()
	if f == "" {
		return http.Get(src) //nolint:gosec,noctx
	}

	u, err := url.Parse(src)
	if err != nil {
		return nil, err
	}

	netrc, err := netrc.Parse(f)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		return nil, err
	}

	if m := netrc.Machine(u.Hostname()); m != nil {
		req.SetBasicAuth(m.Get("login"), m.Get("password"))
	}

	return http.DefaultClient.Do(req)
}

func netrcPath() string {
	if f := os.Getenv("NETRC"); f != "" {
		return f
	}

	usr, err := user.Current()
	if err != nil {
		return ""
	}

	if f := filepath.Join(usr.HomeDir, ".netrc"); fileExists(f) {
		return f
	}

	if runtime.GOOS == "windows" {
		if f := filepath.Join(usr.HomeDir, "_netrc"); fileExists(f) {
			return f
		}
	}

	return ""
}

func fileExists(name string) bool {
	fi, err := os.Stat(name)

	return err == nil && !fi.IsDir() || !errors.Is(err, os.ErrNotExist)
}
