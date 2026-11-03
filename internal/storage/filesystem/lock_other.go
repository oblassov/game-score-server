//go:build !unix

package filesystem

import "os"

// Without flock only the in-process lock protects the db file.

func lockShared(*os.File) error { return nil }

func lockExclusive(*os.File) error { return nil }

func unlock(*os.File) error { return nil }
