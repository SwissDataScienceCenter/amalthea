package common

import (
	"bytes"
	"errors"
	"os"
	"path"
	"strings"

	"github.com/fernet/fernet-go"
	"github.com/labstack/gommon/log"
	"gopkg.in/ini.v1"
)

const UserSecretProxyFolder = "/secrets-user"
const DataConnectorSecretsProxyFolder = "/secrets-dcs"

type DataConnector struct {
	root string

	Name       string
	Remote     string
	RemotePath string
	MountOpt   string
	VfsOpt     string
}

func (dc *DataConnector) fernetKey() (*fernet.Key, error) {
	// the fernet key is mounted as part of the data source secret
	if encodedKey, err := os.ReadFile(path.Join(dc.root, dc.Name, "secretKey")); err == nil {
		return fernet.DecodeKey(string(encodedKey))
	} else {
		return nil, err
	}
}

func (dc *DataConnector) userSecrets() (map[string][]byte, error) {
	var err error

	var fernetKey *fernet.Key
	fernetKey, err = dc.fernetKey()
	if err != nil {
		return nil, err
	}

	var dirEntries []os.DirEntry
	userSecretMountPoint := path.Join(UserSecretProxyFolder, dc.Name)
	if dirEntries, err = os.ReadDir(userSecretMountPoint); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	decodedSecrets := make(map[string][]byte)
	for _, dir := range dirEntries {
		// TODO: Add support for hierarchies ?
		if dir.IsDir() || strings.HasPrefix(dir.Name(), "..") {
			continue
		}

		var content []byte
		if content, err = os.ReadFile(path.Join(userSecretMountPoint, dir.Name())); err != nil {
			return nil, err
		}
		decodedSecrets[dir.Name()] = fernet.VerifyAndDecrypt(content, 0, []*fernet.Key{fernetKey})
	}

	return decodedSecrets, nil
}

func (dc *DataConnector) ConfigData() (map[string][]byte, error) {
	var configData map[string][]byte

	// name is generated with fmt.Sprintf("%s%s-ds-%d", prefix, as.Name, ids)] and maps to pv.SecretRef.Name
	content, err := os.ReadFile(path.Join(dc.root, dc.Name, "configData"))
	if err != nil {
		return nil, err
	}

	iniData, err := ini.Load(content)
	if err != nil {
		return nil, err
	}

	section := iniData.Section(dc.Remote)

	userSecrets, err := dc.userSecrets()
	if err != nil {
		log.Warnf("#### UserSecrets returned %v", err)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Override values with secrets
	for k, v := range userSecrets {
		log.Warnf("#### ConfigData key %v => %v", k, v)
		switch k {
		case "pass":
			// Put it in a file, which will allow for passing to `rclone obscure -` before being placed in the config file.
			configData[k] = v
		default:
			section.Key(k).SetValue(string(v))
		}
	}

	buffer := new(bytes.Buffer)
	if _, err = iniData.WriteTo(buffer); err != nil {
		log.Warnf("#### WriteTo returned %v '%v'", err, buffer)

		return nil, err
	}
	configData["configData"] = buffer.Bytes()
	configData["remote"] = []byte(dc.Remote)
	configData["remotePath"] = []byte(dc.RemotePath)
	if len(dc.MountOpt) > 0 {
		configData["mountOpt"] = []byte(dc.MountOpt)
	}
	if len(dc.VfsOpt) > 0 {
		configData["vfsOpt"] = []byte(dc.VfsOpt)
	}
	// TODO: Write read-only extra flag from "DATA_SOURCES"

	return configData, err
}

func LoadDataSource(root, name string) (*DataConnector, error) {
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

	ds := &DataConnector{
		root,
		name,
		remote,
		remotePath,
		mountOpt,
		vfsOpt,
	}
	return ds, nil
}
