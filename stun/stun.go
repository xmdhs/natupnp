package stun

import (
	"context"
	"fmt"

	"github.com/pion/stun"
)

func GetMappedAddress(ctx context.Context, coon stun.Connection) (stun.XORMappedAddress, error) {
	c, err := stun.NewClient(coon)
	if err != nil {
		return stun.XORMappedAddress{}, fmt.Errorf("GetMappedAddress: %w", err)
	}
	defer c.Close()

	var eErr error
	var xorAddr stun.XORMappedAddress
	if err = c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			eErr = res.Error
			return
		}
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			eErr = getErr
			return
		}
	}); err != nil {
		return stun.XORMappedAddress{}, fmt.Errorf("GetMappedAddress: %w", err)
	}
	if eErr != nil {
		return stun.XORMappedAddress{}, fmt.Errorf("GetMappedAddress: %w", err)
	}
	return xorAddr, nil
}
