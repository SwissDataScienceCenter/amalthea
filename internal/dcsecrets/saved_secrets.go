package dcsecrets

import (
	v1 "k8s.io/api/core/v1"

	"github.com/fernet/fernet-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/ini.v1"
)

// func smth() {
//
// if savedPvcSecret != nil {
// 		if savedSecrets, err := decryptSecrets(flags, savedPvcSecret); err != nil {
// 			klog.Errorf("cannot decode saved storage secrets: %s", err)
// 		} else {
// 			if modifiedConfigData, err := updateConfigData(remote, configData, savedSecrets); err == nil {
// 				configData = modifiedConfigData
// 			} else {
// 				klog.Errorf("cannot update config data: %s", err)
// 			}
// 		}
// 	}
//
// 	return remote, remotePath, configData, flags, nil
// }
// }

func decryptSecrets(flags map[string]string, savedPvcSecret *v1.Secret) (map[string]string, error) {
	savedSecrets := make(map[string]string)

	userSecretKey, ok := flags["secretKey"]
	if !ok {
		return savedSecrets, status.Error(codes.InvalidArgument, "missing user secret key")
	}
	fernetKey, err := fernet.DecodeKey(userSecretKey)
	if err != nil {
		return savedSecrets, status.Errorf(codes.InvalidArgument, "cannot decode user secret key: %s", err)
	}

	if len(savedPvcSecret.Data) > 0 {
		for k, v := range savedPvcSecret.Data {
			savedSecrets[k] = string(fernet.VerifyAndDecrypt([]byte(v), 0, []*fernet.Key{fernetKey}))
		}
	}

	return savedSecrets, nil
}
