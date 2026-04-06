package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
)

var version = "dev"

type Release struct {
	TagName string `json:"tag_name"`
}

func upgrade() {
	repo := "tofunmiadewuyi/dbq"

	// Get latest release
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var rel Release
	json.NewDecoder(resp.Body).Decode(&rel)

	if rel.TagName == version {
		fmt.Println("Already up to date.")
		return
	}

	fmt.Println("Upgrading to", rel.TagName)

	osName := runtime.GOOS
	arch := runtime.GOARCH

	filename := fmt.Sprintf("dbq_%s_%s_%s.tar.gz", rel.TagName, osName, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, rel.TagName, filename)

	tmpFile := "/tmp/dbq.tar.gz"

	out, _ := os.Create(tmpFile)
	defer out.Close()

	resp, err = http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	io.Copy(out, resp.Body)

	// Extract
	f, _ := os.Open(tmpFile)
	gzr, _ := gzip.NewReader(f)
	tr := tar.NewReader(gzr)

	var binPath string = "/tmp/dbq_new"

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == "dbq" {
			outFile, _ := os.Create(binPath)
			io.Copy(outFile, tr)
			outFile.Close()
			break
		}
	}

	os.Chmod(binPath, 0755)

	current, _ := os.Executable()

	err = os.Rename(binPath, current)
	if err != nil {
		fmt.Println("Permission denied. Try running with sudo.")
		return
	}

	fmt.Println("Upgrade complete.")
}
