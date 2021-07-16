package ipfs

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile
var CurrentCommit string

// CurrentVersionNumber is the current application's version literal
const CurrentVersionNumber = "0.9.1"

const ApiVersion = "/go-ipfs/" + CurrentVersionNumber + "/"

// UserAgent is the libp2p user agent used by go-ipfs.
//
// Note: This will end in `/` when no commit is available. This is expected.
var UserAgent = "go-ipfs/" + CurrentVersionNumber + "/" + CurrentCommit
