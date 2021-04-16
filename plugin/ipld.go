package plugin

import (
	"github.com/ipfs/go-ipfs/core/coredag"

	ipld "github.com/ipfs/go-ipld-format"
)

// PluginIPLD is an interface that can be implemented to add handlers for
// for different IPLD formats
// Deprecated: Codecs can now be registered directly in a Plugin's Init method
// using github.com/ipld/go-ipld-prime/multicodec.RegisterEncoder and
// github.com/ipld/go-ipld-prime/multicodec.RegisterDecoder.
type PluginIPLD interface {
	Plugin

	RegisterBlockDecoders(dec ipld.BlockDecoder) error
	RegisterInputEncParsers(iec coredag.InputEncParsers) error
}
