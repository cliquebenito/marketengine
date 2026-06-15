package domain

import "fmt"

type Asset string

const (
	AssetBTC    Asset = "BTC"
	AssetETH    Asset = "ETH"
	AssetGlobal Asset = "GLOBAL"
)

func (a Asset) String() string { return string(a) }

func (a Asset) Validate() error {
	switch a {
	case AssetBTC, AssetETH, AssetGlobal:
		return nil
	}
	return fmt.Errorf("invalid asset %q (want BTC|ETH|GLOBAL)", a)
}

func (a Asset) IsTradeable() bool {
	return a == AssetBTC || a == AssetETH
}

func AssetsTradeable() []Asset { return []Asset{AssetBTC, AssetETH} }
