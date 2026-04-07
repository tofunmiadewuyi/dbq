package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

var version = "dev"

func replaceBinary(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

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


	fmt.Printf("Update available: %s (current: %s)\n", rel.TagName, version)
	fmt.Print("Continue? (y/n): ")

	var input string
	fmt.Scanln(&input)

	if input != "y" && input != "Y" {
		fmt.Println("Aborted.")
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

	var binPath = "/tmp/dbq_new"

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

	err = replaceBinary(binPath, current)
	if err != nil {
		fallback := filepath.Join(os.Getenv("HOME"), ".local/bin/dbq")
		os.MkdirAll(filepath.Dir(fallback), 0755)
		err2 := replaceBinary(binPath, fallback)
		if err2 != nil {
			fmt.Println("Upgrade failed:", err2)
			return
		}
		fmt.Printf("Installed to %s (original location was not writable).\n", fallback)
		fmt.Println("Ensure ~/.local/bin is in your $PATH.")
		return
	}

	fmt.Println("Upgrade complete.")
}
