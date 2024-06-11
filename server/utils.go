package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const EOL = "\n"

var (
	regexpFullVersion = regexp.MustCompile(`^\d+\.\d+\.\d+[\w\.\+\-]*$`)
	regexpLocPath     = regexp.MustCompile(`:\d+:\d+$`)
	regexpJSIdent     = regexp.MustCompile(`^[a-zA-Z_$][\w$]*$`)
	regexpGlobalIdent = regexp.MustCompile(`__[a-zA-Z]+\$`)
	regexpVarEqual    = regexp.MustCompile(`var ([\w$]+)\s*=\s*[\w$]+$`)
)

// isHttpSepcifier returns true if the import path is a remote URL.
func isHttpSepcifier(importPath string) bool {
	return strings.HasPrefix(importPath, "https://") || strings.HasPrefix(importPath, "http://")
}

// isRelativeSpecifier returns true if the import path is a local path.
func isRelativeSpecifier(importPath string) bool {
	return strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") || importPath == "." || importPath == ".."
}

// semverLessThan returns true if the version a is less than the version b.
func semverLessThan(a string, b string) bool {
	return semver.MustParse(a).LessThan(semver.MustParse(b))
}

// includes returns true if the given string is included in the given array.
func includes(a []string, s string) bool {
	if len(a) == 0 {
		return false
	}
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

// endsWith returns true if the given string ends with any of the suffixes.
func endsWith(s string, suffixs ...string) bool {
	for _, suffix := range suffixs {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

// existsDir returns true if the given path is a directory.
func existsDir(filepath string) bool {
	fi, err := os.Lstat(filepath)
	return err == nil && fi.IsDir()
}

// existsFile returns true if the given path is a file.
func existsFile(filepath string) bool {
	fi, err := os.Lstat(filepath)
	return err == nil && !fi.IsDir()
}

// ensureDir creates a directory if it does not exist.
func ensureDir(dir string) (err error) {
	_, err = os.Lstat(dir)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
	}
	return
}

// relPath returns a relative path from the base path to the target path.
func relPath(basePath, targetPath string) (string, error) {
	rp, err := filepath.Rel(basePath, targetPath)
	if err == nil && !isRelativeSpecifier(rp) {
		rp = "./" + rp
	}
	return rp, err
}

// findFiles returns a list of files in the given directory.
func findFiles(root string, dir string, fn func(p string) bool) ([]string, error) {
	rootDir, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		path := name
		if dir != "" {
			path = dir + "/" + name
		}
		if entry.IsDir() {
			if name == "node_modules" {
				continue
			}
			subFiles, err := findFiles(filepath.Join(rootDir, name), path, fn)
			if err != nil {
				return nil, err
			}
			n := len(files)
			files = make([]string, n+len(subFiles))
			for i, f := range subFiles {
				files[i+n] = f
			}
			copy(files, subFiles)
		} else {
			if fn(path) {
				files = append(files, path)
			}
		}
	}
	return files, nil
}

// btoaUrl converts a string to a base64 string.
func btoaUrl(s string) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(s)), "=")
}

// atobUrl converts a base64 string to a string.
func atobUrl(s string) (string, error) {
	if l := len(s) % 4; l > 0 {
		s += strings.Repeat("=", 4-l)
	}
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// removeHttpUrlProtocol removes the `http[s]:` protocol from the given url.
func removeHttpUrlProtocol(url string) string {
	if strings.HasPrefix(url, "https://") {
		return url[6:]
	}
	if strings.HasPrefix(url, "http://") {
		return url[5:]
	}
	return url
}

// toEnvName converts the given string to an environment variable name.
func toEnvName(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') {
			runes[i] = r
		} else if r >= 'a' && r <= 'z' {
			runes[i] = r - 'a' + 'A'
		} else {
			runes[i] = '_'
		}
	}
	return string(runes)
}

// concatBytes concatenates two byte slices.
func concatBytes(a, b []byte) []byte {
	c := make([]byte, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)
	return c
}

// mustEncodeJSON encodes the given value to a JSON byte slice.
func mustEncodeJSON(v interface{}) []byte {
	buf := bytes.NewBuffer(nil)
	err := json.NewEncoder(buf).Encode(v)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// parseJSONFile parses the given JSON file and stores the result in the value pointed to by v.
func parseJSONFile(filename string, v interface{}) (err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(v)
}

// run executes the given command and returns the output.
func run(cmd string, args ...string) (output []byte, err error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	c := exec.Command(cmd, args...)
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	err = c.Run()
	if err != nil {
		if errBuf.Len() > 0 {
			err = fmt.Errorf("%s: %s", err, errBuf.String())
		}
		return
	}
	if errBuf.Len() > 0 {
		err = fmt.Errorf("%s", errBuf.String())
		return
	}
	output = outBuf.Bytes()
	return
}
