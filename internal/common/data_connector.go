package common

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"

	"github.com/SwissDataScienceCenter/amalthea/internal/kube"
	"github.com/fernet/fernet-go"
	"gopkg.in/ini.v1"
	v1 "k8s.io/api/core/v1"
)

const UserSecretProxyFolder = "/secrets-user"
const DataConnectorSecretsProxyFolder = "/secrets-dcs"

type DataConnector struct {
	root       string
	name       string
	secretName string

	Remote     string
	RemotePath string
	MountOpt   string
	VfsOpt     string
}

func (dc *DataConnector) fernetKey() (*fernet.Key, error) {
	// the fernet key is mounted as part of the data source secret
	if encodedKey, err := os.ReadFile(path.Join(dc.root, dc.name, "secretKey")); err == nil {
		return fernet.DecodeKey(string(encodedKey))
	} else {
		return nil, err
	}
}

func (dc *DataConnector) userSecrets(ctx context.Context) (map[string]string, error) {
	var err error

	var fernetKey *fernet.Key
	fernetKey, err = dc.fernetKey()
	if err != nil {
		return nil, err
	}

	var secret *v1.Secret
	if secret, err = kube.Secret(ctx, dc.secretName); err != nil {
		return nil, err
	}

	decodedSecrets := make(map[string]string)
	for k, v := range secret.Data {
		decodedSecrets[k] = string(fernet.VerifyAndDecrypt(v, 0, []*fernet.Key{fernetKey}))
	}
	return decodedSecrets, nil
}

func (dc *DataConnector) ConfigData(ctx context.Context) (*string, error) {
	// name is generated with fmt.Sprintf("%s%s-ds-%d", prefix, as.Name, ids)] and maps to pv.SecretRef.Name
	content, err := os.ReadFile(path.Join(dc.root, dc.name, "configData"))
	if err != nil {
		return nil, err
	}

	iniData, err := ini.Load(content)
	if err != nil {
		return nil, err
	}

	section := iniData.Section(dc.Remote)

	var userSecrets map[string]string
	userSecrets, err = dc.userSecrets(ctx)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Override values with secrets
	for k, v := range userSecrets {
		section.Key(k).SetValue(v)
	}

	buffer := new(bytes.Buffer)
	if _, err = iniData.WriteTo(buffer); err != nil {
		return nil, err
	}

	// So we can take the address...
	b := buffer.String()
	return &b, err
}

func LoadDataSource(root, name string) (*DataConnector, error) {
	var content []byte
	var err error

	if content, err = os.ReadFile(path.Join(UserSecretProxyFolder, name)); err != nil {
		return nil, err
	}
	secretName := string(content)

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
		secretName,
		remote,
		remotePath,
		mountOpt,
		vfsOpt,
	}
	return ds, nil
}
