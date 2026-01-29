package observer

import "context"

type Chain struct {
	Items []Observer
}

func (c Chain) OnComplete(ctx context.Context, call ProviderCall) error {
	for _, o := range c.Items {
		if o == nil {
			continue
		}
		_ = o.OnComplete(ctx, call)
	}
	return nil
}
