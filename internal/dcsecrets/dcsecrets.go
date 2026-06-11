package dcsecrets

import (
	"io/fs"
	"os"
	"path/filepath"
)

func CombineRcloneConfigs(location string, output string) error {
	files := []string{}
	filepath.WalkDir(location, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})

	fout, err := os.Create(output)
	if err != nil {
		return err
	}
	defer fout.Close()

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		_, err = fout.WriteString("\n")
		if err != nil {
			return err
		}
		_, err = fout.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}
