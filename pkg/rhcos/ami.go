package rhcos

import (
	"context"

	"github.com/pkg/errors"
)

// AMI fetches the HVM AMI ID of a Red Hat CoreOS release.  If 'build'
// is an empty string, the latest build in the given channel will be
// used.
func AMI(ctx context.Context, channel, build, region string) (string, error) {
	meta, err := fetchMetadata(ctx, channel, build)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch RHCOS metadata")
	}

	for _, ami := range meta.AMIs {
		if ami.Name == region {
			return ami.HVM, nil
		}
	}

	return "", errors.Errorf("no RHCOS AMIs found in %s", region)
}
