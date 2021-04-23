export GOPROXY=direct
export BS_CFG_LOG=info
export BS_CFG_TIMEOUT=5
export BS_CFG_CID_REQ_NUM=10
make build && ./cmd/ipfs/ipfs daemon
