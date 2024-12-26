package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/ije/gox/valid"
)

// isHttpSepcifier returns true if the specifier is a remote URL.
func isHttpSepcifier(specifier string) bool {
	return strings.HasPrefix(specifier, "https://") || strings.HasPrefix(specifier, "http://")
}

// isRelPathSpecifier returns true if the specifier is a local path.
func isRelPathSpecifier(specifier string) bool {
	return strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../")
}

// isAbsPathSpecifier returns true if the specifier is an absolute path.
func isAbsPathSpecifier(specifier string) bool {
	return strings.HasPrefix(specifier, "/") || strings.HasPrefix(specifier, "file://")
}

// isJsModuleSpecifier returns true if the specifier is a json module.
func isJsonModuleSpecifier(specifier string) bool {
	if !strings.HasSuffix(specifier, ".json") {
		return false
	}
	_, _, subpath, _ := splitEsmPath(specifier)
	return subpath != "" && strings.HasSuffix(subpath, ".json")
}

// isJsModuleSpecifier checks if the given specifier is a node.js built-in module.
func isNodeBuiltInModule(specifier string) bool {
	return strings.HasPrefix(specifier, "node:") && nodeBuiltinModules[specifier[5:]]
}

// normalizeImportSpecifier normalizes the given specifier.
func normalizeImportSpecifier(specifier string) string {
	specifier = strings.TrimPrefix(specifier, "npm:")
	specifier = strings.TrimPrefix(specifier, "./node_modules/")
	if specifier == "." {
		specifier = "./index"
	} else if specifier == ".." {
		specifier = "../index"
	}
	if nodeBuiltinModules[specifier] {
		return "node:" + specifier
	}
	return specifier
}

// semverLessThan returns true if the version a is less than the version b.
func semverLessThan(a string, b string) bool {
	return semver.MustParse(a).LessThan(semver.MustParse(b))
}

// checks if the given hostname is a local address.
func isLocalhost(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1" || (valid.IsIPv4(hostname) && strings.HasPrefix(hostname, "192.168."))
}

// isCommitish returns true if the given string is a commit hash.
func isCommitish(s string) bool {
	return len(s) >= 7 && len(s) <= 40 && valid.IsHexString(s) && containsDigit(s)
}

// isJsReservedWord returns true if the given string is a reserved word in JavaScript.
func isJsReservedWord(word string) bool {
	switch word {
	case "abstract", "arguments", "await", "boolean", "break", "byte", "case", "catch", "char", "class", "const", "continue", "debugger", "default", "delete", "do", "double", "else", "enum", "eval", "export", "extends", "false", "final", "finally", "float", "for", "function", "goto", "if", "implements", "import", "in", "instanceof", "int", "interface", "let", "long", "native", "new", "null", "package", "private", "protected", "public", "return", "short", "static", "super", "switch", "synchronized", "this", "throw", "throws", "transient", "true", "try", "typeof", "var", "void", "volatile", "while", "with", "yield":
		return true
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

// contains returns true if the given string is included in the given array.
func contains(a []string, s string) bool {
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

// containsDigit returns true if the given string contains a digit.
func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
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
	if err == nil && !isRelPathSpecifier(rp) {
		rp = "./" + rp
	}
	return rp, err
}

// findFiles returns a list of files in the given directory.
func findFiles(root string, dir string, filter func(filename string) bool) ([]string, error) {
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
		filename := name
		if dir != "" {
			filename = dir + "/" + name
		}
		if entry.IsDir() {
			if name == "node_modules" {
				continue
			}
			subFiles, err := findFiles(filepath.Join(rootDir, name), filename, filter)
			if err != nil {
				return nil, err
			}
			newFiles := make([]string, len(files)+len(subFiles))
			copy(newFiles, files)
			copy(newFiles[len(files):], subFiles)
			files = newFiles
		} else {
			if filter(filename) {
				files = append(files, filename)
			}
		}
	}
	return files, nil
}

// btoaUrl converts a string to a base64 string.
func btoaUrl(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// atobUrl converts a base64 string to a string.
func atobUrl(s string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// appendVaryHeader appends the given key to the `Vary` header.
func appendVaryHeader(header http.Header, key string) {
	vary := header.Get("Vary")
	if vary == "" {
		header.Set("Vary", key)
	} else {
		header.Set("Vary", vary+", "+key)
	}
}

var bufferPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

// NewBuffer returns a new buffer from the buffer pool.
func NewBuffer() (buffer *bytes.Buffer, recycle func()) {
	buf := bufferPool.Get().(*bytes.Buffer)
	return buf, func() {
		buf.Reset()
		bufferPool.Put(buf)
	}
}

// concatBytes concatenates two byte slices.
func concatBytes(a, b []byte) []byte {
	al, bl := len(a), len(b)
	if al == 0 {
		return b[0:]
	}
	if bl == 0 {
		return a[0:]
	}
	c := make([]byte, al+bl)
	copy(c, a)
	copy(c[al:], b)
	return c
}

// run executes the given command and returns the output.
func run(cmd string, args ...string) (output []byte, err error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	c := exec.Command(cmd, args...)
	c.Dir = os.TempDir()
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	err = c.Run()
	if err != nil {
		if errBuf.Len() > 0 {
			err = errors.New(errBuf.String())
		}
		return
	}
	output = outBuf.Bytes()
	return
}
