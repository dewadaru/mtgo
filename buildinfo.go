package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"runtime/debug"
	"slices"
	"strconv"
	"time"
)

var version = "dev" // has to be set by ldflags

const (
	buildInfoModuleStart byte = iota
	buildInfoModuleFinish
	buildInfoModuleDelimeter
)

func getVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	// Pre-allocate with expected capacity
	date := time.Now()
	commit := ""
	goVersion := buildInfo.GoVersion
	dirtySuffix := ""

	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.time":
			date, _ = time.Parse(time.RFC3339, setting.Value)
		case "vcs.revision":
			commit = setting.Value
		case "vcs.modified":
			if dirty, _ := strconv.ParseBool(setting.Value); dirty {
				dirtySuffix = " [dirty]"
			}
		}
	}

	hasher := sha256.New()
	buf := make([]byte, 8) // Reusable buffer for binary.Write

	checksumModule := func(mod *debug.Module) {
		hasher.Write([]byte{buildInfoModuleStart})
		hasher.Write([]byte(mod.Path))
		hasher.Write([]byte{buildInfoModuleDelimeter})
		hasher.Write([]byte(mod.Version))
		hasher.Write([]byte{buildInfoModuleDelimeter})
		hasher.Write([]byte(mod.Sum))
		hasher.Write([]byte{buildInfoModuleFinish})
	}

	hasher.Write([]byte(buildInfo.Path))

	// Use pre-allocated buffer instead of binary.Write
	binary.LittleEndian.PutUint64(buf, uint64(1+len(buildInfo.Deps)))
	hasher.Write(buf)

	// Use slices.SortFunc from Go 1.21+ (more efficient)
	slices.SortFunc(buildInfo.Deps, func(a, b *debug.Module) int {
		if a.Path > b.Path {
			return -1
		}
		if a.Path < b.Path {
			return 1
		}
		return 0
	})

	checksumModule(&buildInfo.Main)

	for _, module := range buildInfo.Deps {
		checksumModule(module)
	}

	// Pre-calculate checksum to avoid allocation in Sprintf
	checksum := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	return fmt.Sprintf("%s (%s: %s on %s%s, modules checksum %s)",
		version,
		goVersion,
		date.Format(time.RFC3339),
		commit,
		dirtySuffix,
		checksum)
}