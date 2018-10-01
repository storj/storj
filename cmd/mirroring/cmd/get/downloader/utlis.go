package downloader

import (
	"path"
	"os"
)

const EMPTY_STRING = ""

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

func StripPrefix(name string) string {
	_, file := path.Split(name)
	return file
}

type pathResolver func(dir, file, cwd, objName string) string

var fileResolver = func(dir, file, cwd, objName string) string {
	if dir != EMPTY_STRING && file != EMPTY_STRING {
		return path.Join(dir, file)
	}
	if dir != EMPTY_STRING {
		return path.Join(dir, objName)
	}
	if file != EMPTY_STRING {
		return path.Join(cwd, file)
	}

	return path.Join(cwd, objName)
}

var folderResolver = func(dir, file, cwd, objName string) string {
	if dir != EMPTY_STRING {
		return path.Join(dir, objName)
	}

	return path.Join(cwd, objName)
}

func ResolveFilePathForObject(filePath, cwd, objName string) string {
	return resolveFilePath(filePath, cwd, objName, fileResolver)
}


func ResolveFilePathForFolder(filePath, cwd, objName string) string  {
	return resolveFilePath(filePath, cwd, objName, folderResolver)
}

func resolveFilePath(filePath, cwd, objName string, resolver pathResolver) string {
	if filePath == EMPTY_STRING {
		return path.Join(cwd, objName)
	}

	dir, file := path.Split(filePath)
	return resolver(dir, file, cwd, objName)
}
