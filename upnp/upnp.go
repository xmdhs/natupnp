package upnp

import (
	"context"
	"fmt"

	"github.com/huin/goupnp/dcps/ocf/internetgateway2"
	"golang.org/x/sync/errgroup"
)

func AddPortMapping(ctx context.Context,
	NewRemoteHost string,
	NewExternalPort uint16,
	NewProtocol string,
	NewInternalPort uint16,
	NewInternalClient string,
	NewEnabled bool,
	NewPortMappingDescription string,
	NewLeaseDuration uint32,
) error {
	clients, err := pickRouterClient(ctx)
	if err != nil {
		return fmt.Errorf("AddPortMapping: %w", err)
	}

	tasks, _ := errgroup.WithContext(ctx)

	for _, v := range clients {
		v := v
		tasks.Go(func() error {
			return v.AddPortMapping(NewRemoteHost, NewExternalPort, NewProtocol, NewInternalPort, NewInternalClient, NewEnabled, NewPortMappingDescription, NewLeaseDuration)
		})
	}

	if err := tasks.Wait(); err != nil {
		return fmt.Errorf("AddPortMapping: %w", err)
	}

	return nil
}

type routerClient interface {
	AddPortMapping(
		NewRemoteHost string,
		NewExternalPort uint16,
		NewProtocol string,
		NewInternalPort uint16,
		NewInternalClient string,
		NewEnabled bool,
		NewPortMappingDescription string,
		NewLeaseDuration uint32,
	) (err error)

	GetExternalIPAddress() (
		NewExternalIPAddress string,
		err error,
	)
}

func pickRouterClient(ctx context.Context) ([]routerClient, error) {
	tasks, _ := errgroup.WithContext(ctx)
	var ip1Clients []*internetgateway2.WANIPConnection1
	tasks.Go(func() error {
		var err error
		ip1Clients, _, err = internetgateway2.NewWANIPConnection1Clients()
		return err
	})
	var ip2Clients []*internetgateway2.WANIPConnection2
	tasks.Go(func() error {
		var err error
		ip2Clients, _, err = internetgateway2.NewWANIPConnection2Clients()
		return err
	})
	var ppp1Clients []*internetgateway2.WANPPPConnection1
	tasks.Go(func() error {
		var err error
		ppp1Clients, _, err = internetgateway2.NewWANPPPConnection1Clients()
		return err
	})

	if err := tasks.Wait(); err != nil {
		return nil, fmt.Errorf("pickRouterClient: %w", err)
	}

	switch {
	case len(ip2Clients) >= 1:
		return any2slice[routerClient](ip2Clients), nil
	case len(ip1Clients) >= 1:
		return any2slice[routerClient](ip1Clients), nil
	case len(ppp1Clients) >= 1:
		return any2slice[routerClient](ppp1Clients), nil
	default:
		return nil, nil
	}
}

func any2slice[T, E any](list []E) []T {
	nl := make([]T, 0, len(list))
	for _, v := range list {
		nl = append(nl, any(v).(T))
	}
	return nl
}
