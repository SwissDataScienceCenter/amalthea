package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fernet/fernet-go"
	"gopkg.in/ini.v1"
)

const LocalSessionSecretsPath = "/secrets-session"
const LocalDataConnectorPath = "/secrets-dcs"
const LocalDataConnectorSecretPath = "/secrets-dcs-secrets"

type DataConnector struct {
	Root string

	Name   string
	Remote string

	RemotePath string
	MountOpt   string
	VfsOpt     string

	ExtraArgs []string
}

// vfsOptFlags maps the JSON field names of csi-rclone's VfsOpt struct to the
// rclone CLI flags they correspond to. The mapping is not derivable from the
// field name: several VFS options have no "vfs" prefix and some differ from the
// flag name entirely (e.g. ChunkSize -> --vfs-read-chunk-size).
// See csi-rclone/pkg/rclone/rclone.go and rclone's vfs/vfscommon/options.go.
var vfsOptFlags = map[string]string{
	"NoSeek":             "--no-seek",
	"NoChecksum":         "--no-checksum",
	"ReadOnly":           "--read-only",
	"NoModTime":          "--no-modtime",
	"DirCacheTime":       "--dir-cache-time",
	"Refresh":            "--vfs-refresh",
	"PollInterval":       "--poll-interval",
	"Umask":              "--umask",
	"UID":                "--uid",
	"GID":                "--gid",
	"DirPerms":           "--dir-perms",
	"FilePerms":          "--file-perms",
	"ChunkSize":          "--vfs-read-chunk-size",
	"ChunkSizeLimit":     "--vfs-read-chunk-size-limit",
	"ChunkStreams":       "--vfs-read-chunk-streams",
	"CacheMode":          "--vfs-cache-mode",
	"CacheMaxAge":        "--vfs-cache-max-age",
	"CacheMaxSize":       "--vfs-cache-max-size",
	"CacheMinFreeSpace":  "--vfs-cache-min-free-space",
	"CachePollInterval":  "--vfs-cache-poll-interval",
	"CaseInsensitive":    "--vfs-case-insensitive",
	"WriteWait":          "--vfs-write-wait",
	"ReadWait":           "--vfs-read-wait",
	"WriteBack":          "--vfs-write-back",
	"ReadAhead":          "--vfs-read-ahead",
	"UsedIsSize":         "--vfs-used-is-size",
	"FastFingerprint":    "--vfs-fast-fingerprint",
	"DiskSpaceTotalSize": "--vfs-disk-space-total-size",
}

// mountOptFlags maps the JSON field names of csi-rclone's MountOpt struct to the
// rclone CLI flags they correspond to.
// See csi-rclone/pkg/rclone/rclone.go and rclone's cmd/mountlib/mount.go.
var mountOptFlags = map[string]string{
	"DebugFUSE":          "--debug-fuse",
	"AllowNonEmpty":      "--allow-non-empty",
	"AllowRoot":          "--allow-root",
	"AllowOther":         "--allow-other",
	"DefaultPermissions": "--default-permissions",
	"WritebackCache":     "--write-back-cache",
	"DaemonWait":         "--daemon-wait",
	"MaxReadAhead":       "--max-read-ahead",
	"ExtraOptions":       "--option",
	"ExtraFlags":         "--fuse-flag",
	"AttrTimeout":        "--attr-timeout",
	"DeviceName":         "--devname",
	"VolumeName":         "--volname",
	"NoAppleDouble":      "--noappledouble",
	"NoAppleXattr":       "--noapplexattr",
	"AsyncRead":          "--async-read",
	"CaseInsensitive":    "--mount-case-insensitive",
}

// Default rclone options applied to a data connector mount when the secret does
// not set them. Mirrors csi-rclone's defaults. Applying them here keeps a single
// source of truth and avoids passing the same option twice.
var (
	mountOptDefaults = map[string]any{"AllowNonEmpty": true}
	vfsOptDefaults   = map[string]any{"CacheMode": "writes"}
)

// rcloneArgsFromJSON converts a flat JSON object of rclone options (the format
// of the csi-rclone `vfsOpt`/`mountOpt` secrets) into rclone CLI arguments,
// applying any default the secret does not override. Unknown keys are logged
// and skipped so a typo never becomes a bogus flag.
func rcloneArgsFromJSON(raw string, flags map[string]string, defaults map[string]any) (string, error) {
	opts := map[string]any{}
	if raw = strings.TrimSpace(raw); raw != "" {
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&opts); err != nil {
			return "", fmt.Errorf("could not parse rclone options %q: %w", raw, err)
		}
	}
	for key, value := range defaults {
		if _, ok := opts[key]; !ok {
			opts[key] = value
		}
	}
	keys := slices.Sorted(maps.Keys(opts))
	args := make([]string, 0, len(opts))
	for _, key := range keys {
		flag, ok := flags[key]
		if !ok {
			slog.Warn("ignoring unknown rclone option", "option", key)
			continue
		}
		switch value := opts[key].(type) {
		case bool:
			args = append(args, fmt.Sprintf("%s=%t", flag, value))
		case string:
			args = append(args, fmt.Sprintf("%s=%s", flag, value))
		case json.Number:
			args = append(args, fmt.Sprintf("%s=%s", flag, value.String()))
		case []any:
			for _, item := range value {
				args = append(args, fmt.Sprintf("%s=%v", flag, item))
			}
		default:
			slog.Warn("ignoring rclone option with unsupported value", "option", key, "value", value)
		}
	}
	// a value containing spaces (e.g. a FUSE option) would split into separate args.
	// Upgrade to array expansion if such values become supported.
	return strings.Join(args, " "), nil
}

func (dc *DataConnector) fernetKey() (*fernet.Key, error) {
	// the fernet key is mounted as part of the data source secret
	root := filepath.FromSlash(filepath.Clean(dc.Root)) // Clean and convert `dc.Root`
	if encodedKey, err := os.ReadFile(filepath.Join(root, dc.Name, "secretKey")); err == nil {
		return fernet.DecodeKey(string(encodedKey))
	} else {
		return nil, err
	}
}

func (dc *DataConnector) dataConnectorSecrets() (map[string][]byte, error) {
	var err error

	var fernetKey *fernet.Key
	fernetKey, err = dc.fernetKey()
	if err != nil {
		return nil, err
	}

	dataConnectorSecretBaseFilePath := filepath.FromSlash(filepath.Clean(LocalDataConnectorSecretPath))
	dataConnectorSecretMountPoint := filepath.Join(dataConnectorSecretBaseFilePath, dc.Name)

	var dirEntries []os.DirEntry
	if dirEntries, err = os.ReadDir(dataConnectorSecretMountPoint); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	decodedSecrets := make(map[string][]byte)
	for _, dir := range dirEntries {
		// TODO: Add support for hierarchies ?
		if dir.IsDir() || strings.HasPrefix(dir.Name(), "..") {
			continue
		}

		var content []byte
		if content, err = os.ReadFile(filepath.Join(dataConnectorSecretMountPoint, dir.Name())); err != nil {
			return nil, err
		}
		decodedSecrets[dir.Name()] = fernet.VerifyAndDecrypt(content, 0, []*fernet.Key{fernetKey})
	}

	return decodedSecrets, nil
}

func (dc *DataConnector) ConfigFiles() (map[string][]byte, error) {
	configFiles := map[string][]byte{}

	root := filepath.FromSlash(filepath.Clean(dc.Root)) // Clean and convert `dc.Root`
	content, err := os.ReadFile(filepath.Join(root, dc.Name, "configData"))
	if err != nil {
		return nil, err
	}

	iniData, err := ini.Load(content)
	if err != nil {
		return nil, err
	}

	section := iniData.Section(dc.Remote)

	dataConnectorSecrets, err := dc.dataConnectorSecrets()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Override values with secrets
	for k, v := range dataConnectorSecrets {
		switch k {
		case "pass":
			// Put it in a file, which will allow for passing to `rclone obscure -` before being placed in the config file.
			configFiles[k] = v
		default:
			section.Key(k).SetValue(string(v))
		}
	}

	buffer := new(bytes.Buffer)
	if _, err = iniData.WriteTo(buffer); err != nil {
		return nil, err
	}
	configFiles["configData"] = buffer.Bytes()
	configFiles["remote"] = []byte(dc.Remote)
	configFiles["remotePath"] = []byte(dc.RemotePath)
	// If an options secret contains malformed JSON data, do not abort: log the issue
	// and mount the connector with the defaults only. (Unknown or misspelled keys
	// parse fine and are logged and skipped in rcloneArgsFromJSON.)
	mountArgs, mountErr := rcloneArgsFromJSON(dc.MountOpt, mountOptFlags, mountOptDefaults)
	if mountErr != nil {
		slog.Warn("ignoring malformed mount options for data connector", "dataConnector", dc.Name, "error", mountErr)
		mountArgs, _ = rcloneArgsFromJSON("", mountOptFlags, mountOptDefaults)
	}
	configFiles["mountArgs"] = []byte(mountArgs)

	vfsArgs, vfsErr := rcloneArgsFromJSON(dc.VfsOpt, vfsOptFlags, vfsOptDefaults)
	if vfsErr != nil {
		slog.Warn("ignoring malformed vfs options for data connector", "dataConnector", dc.Name, "error", vfsErr)
		vfsArgs, _ = rcloneArgsFromJSON("", vfsOptFlags, vfsOptDefaults)
	}
	configFiles["vfsArgs"] = []byte(vfsArgs)
	if len(dc.ExtraArgs) > 0 {
		configFiles["extraArgs"] = []byte(strings.Join(dc.ExtraArgs, " "))
	}

	return configFiles, err
}

// Local definition of the types so that we may unmarshall into typed values
type cSessionSecretRef struct {
	Name  string `json:"name"`
	Key   string `json:"key,omitempty"`
	Adopt bool   `json:"adopt"`
}

type cDataSource struct {
	Type       string             `json:"type,omitempty"`
	MountPath  string             `json:"mountPath,omitempty"`
	AccessMode string             `json:"accessMode,omitempty"`
	SecretRef  *cSessionSecretRef `json:"secretRef,omitempty"`
}

func parsePV(name string) ([]string, error) {
	extraArgs := []string{}
	var content []byte
	var err error

	localSessionSecretsBaseFilePath := filepath.FromSlash(filepath.Clean(LocalSessionSecretsPath))
	if content, err = os.ReadFile(filepath.Join(localSessionSecretsBaseFilePath, name)); err != nil {
		return nil, err
	}

	ds := &cDataSource{}
	if err = json.Unmarshal(content, ds); err != nil {
		return nil, err
	}

	if !strings.Contains(ds.AccessMode, "Write") {
		extraArgs = append(extraArgs, "--read-only")
	}

	return extraArgs, nil
}

func LoadDataConnector(rootDir, name string) (*DataConnector, error) {
	var content []byte
	var err error

	root := filepath.FromSlash(filepath.Clean(rootDir)) // Clean and convert `dc.Root`

	if content, err = os.ReadFile(filepath.Join(root, name, "remote")); err != nil {
		return nil, err
	}
	remote := string(content)

	if content, err = os.ReadFile(filepath.Join(root, name, "remotePath")); err != nil {
		return nil, err
	}
	remotePath := string(content)

	if content, err = os.ReadFile(filepath.Join(root, name, "mountOpt")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	mountOpt := string(content)

	if content, err = os.ReadFile(filepath.Join(root, name, "vfsOpt")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	vfsOpt := string(content)

	var extraArgs []string
	if extraArgs, err = parsePV(name); err != nil {
		return nil, err
	}

	dc := &DataConnector{
		root,
		name,
		remote,
		remotePath,
		mountOpt,
		vfsOpt,
		extraArgs,
	}
	return dc, nil
}
