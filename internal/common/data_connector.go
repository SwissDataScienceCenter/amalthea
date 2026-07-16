package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path"
	"strings"

	"github.com/fernet/fernet-go"
	"gopkg.in/ini.v1"
)

const UserSecretProxyFolder = "/secrets-user"
const DataConnectorProxyFolder = "/secrets-dcs"
const DataConnectorSecretProxyFolder = "/secrets-dcs-secrets"

type DataConnector struct {
	root string

	Name   string
	Remote string

	RemotePath string
	MountOpt   string
	VfsOpt     string

	ExtraArgs []string
}

func (dc *DataConnector) fernetKey() (*fernet.Key, error) {
	// the fernet key is mounted as part of the data source secret
	if encodedKey, err := os.ReadFile(path.Join(dc.root, dc.Name, "secretKey")); err == nil {
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

	var dirEntries []os.DirEntry
	dataConnectorSecretMountPoint := path.Join(DataConnectorSecretProxyFolder, dc.Name)
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
		if content, err = os.ReadFile(path.Join(dataConnectorSecretMountPoint, dir.Name())); err != nil {
			return nil, err
		}
		decodedSecrets[dir.Name()] = fernet.VerifyAndDecrypt(content, 0, []*fernet.Key{fernetKey})
	}

	return decodedSecrets, nil
}

func (dc *DataConnector) ConfigFiles() (map[string][]byte, error) {
	configFiles := map[string][]byte{}

	content, err := os.ReadFile(path.Join(dc.root, dc.Name, "configData"))
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
	if len(dc.MountOpt) > 0 {
		configFiles["mountOpt"] = []byte(dc.MountOpt)
	}
	if len(dc.VfsOpt) > 0 {
		configFiles["vfsOpt"] = []byte(dc.VfsOpt)
	}
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

	if content, err = os.ReadFile(path.Join(UserSecretProxyFolder, name)); err != nil {
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

func LoadDataConnector(root, name string) (*DataConnector, error) {
	var content []byte
	var err error

	if content, err = os.ReadFile(path.Join(root, name, "remote")); err != nil {
		return nil, err
	}
	remote := string(content)

	if content, err = os.ReadFile(path.Join(root, name, "remotePath")); err != nil {
		return nil, err
	}
	remotePath := string(content)

	if content, err = os.ReadFile(path.Join(root, name, "mountOpt")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	mountOpt := string(content)

	if content, err = os.ReadFile(path.Join(root, name, "vfsOpt")); err != nil && !errors.Is(err, os.ErrNotExist) {
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
