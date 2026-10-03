package engine

import (
	"fmt"
	"io"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/reader"
)

func uploadRemote(remote reader.RemoteInputCommandRunner, path, url string) error {
	// Presigned URLs should never contain literal control characters. Reject them
	// without printing the URL rather than allowing extra curl config directives.
	for _, c := range url {
		if c < 0x20 || c == 0x7f {
			return fmt.Errorf("upload URL contains a control character")
		}
	}
	// curl config quoting differs from shell and Go string quoting.
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(url)
	config := strings.NewReader("url = \"" + escaped + "\"\n")
	// -q must be first to disable .curlrc, including implicit verbose/trace output.
	// The URL travels only through stdin, not the shell command or process argv.
	return remote.ExecRemoteCommandWithInput("curl", []string{"-q", "--config", "-", "--globoff", "-s", "-f", "-X", "PUT", "-T", path}, nil, config, io.Discard)
}
