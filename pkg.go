package main

import (
	"os"
	"path/filepath"
)

type packageManager struct {
	Name string
	Args []string
}

// detectPackageManager picks the package manager from a project's lockfile.
// ok is false when root isn't a Node project.
func detectPackageManager(root string) (packageManager, bool) {
	has := func(f string) bool {
		_, err := os.Stat(filepath.Join(root, f))
		return err == nil
	}
	switch {
	case has("pnpm-lock.yaml"):
		return packageManager{"pnpm", []string{"install"}}, true
	case has("yarn.lock"):
		return packageManager{"yarn", []string{"install"}}, true
	case has("bun.lockb"):
		return packageManager{"bun", []string{"install"}}, true
	case has("package-lock.json"):
		return packageManager{"npm", []string{"install"}}, true
	case has("package.json"):
		return packageManager{"npm", []string{"install"}}, true
	}
	return packageManager{}, false
}
