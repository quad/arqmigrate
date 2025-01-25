package object

import (
	"context"

	"github.com/Backblaze/blazer/b2"
)

const clientID = "0024c9131bbec2e0000000006"
const clientKey = "K002zzTVRrQq/3TMvDFRl/zmapV02Og"

func NewClient(ctx context.Context) (*b2.Client, error) {
	client, err := b2.NewClient(ctx, clientID, clientKey)
	if err != nil {
		return nil, err
	}

	return client, nil
}
