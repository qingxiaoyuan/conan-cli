// Package atomicfile 提供原子写入：先写同目录临时文件，sync 后 rename
// 覆盖目标（NFR-03）。项目配置、全局配置、credentials 和 conanfile 的
// 写入都必须走这里，保证任何时刻磁盘上不会出现写了一半的文件。
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write atomically replaces path with data. mode applies to the final file
// (e.g. 0o600 for credentials). The caller must ensure the parent directory
// exists.
func Write(path string, data []byte, mode os.FileMode) error {
	pattern := "." + filepath.Base(path) + ".*"
	temporary, err := os.CreateTemp(filepath.Dir(path), pattern)
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
