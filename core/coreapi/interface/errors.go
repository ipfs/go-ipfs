package iface

import "errors"

var (
	ErrIsDir     = errors.New("object is a directory")
	ErrOffline   = errors.New("can't resolve, ipfs node is offline")
	ErrIsSymLink = errors.New("object is a symbolic link")
)
