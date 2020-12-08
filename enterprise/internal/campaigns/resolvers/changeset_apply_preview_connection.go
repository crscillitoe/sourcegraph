package resolvers

import (
	"context"
	"sync"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	ee "github.com/sourcegraph/sourcegraph/enterprise/internal/campaigns"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

var _ graphqlbackend.ChangesetApplyPreviewConnectionResolver = &changesetApplyPreviewConnectionResolver{}

type changesetApplyPreviewConnectionResolver struct {
	store       *ee.Store
	httpFactory *httpcli.Factory

	opts           ee.GetRewirerMappingsOpts
	campaignSpecID int64

	once     sync.Once
	mappings ee.RewirerMappings
	err      error
}

func (r *changesetApplyPreviewConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	mappings, err := r.compute(ctx)
	if err != nil {
		return 0, err
	}
	return int32(len(mappings)), nil
}

func (r *changesetApplyPreviewConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	return graphqlutil.HasNextPage(false), nil
}

func (r *changesetApplyPreviewConnectionResolver) Nodes(ctx context.Context) ([]graphqlbackend.ChangesetApplyPreviewResolver, error) {
	mappings, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	syncData, err := r.store.ListChangesetSyncData(ctx, ee.ListChangesetSyncDataOpts{ChangesetIDs: mappings.ChangesetIDs()})
	if err != nil {
		return nil, err
	}
	scheduledSyncs := make(map[int64]time.Time)
	for _, d := range syncData {
		scheduledSyncs[d.ChangesetID] = ee.NextSync(time.Now, d)
	}

	resolvers := make([]graphqlbackend.ChangesetApplyPreviewResolver, 0, len(mappings))
	for _, mapping := range mappings {
		resolvers = append(resolvers, &changesetApplyPreviewResolver{
			store:             r.store,
			httpFactory:       r.httpFactory,
			mapping:           mapping,
			preloadedNextSync: scheduledSyncs[mapping.ChangesetID],
		})
	}

	return resolvers, nil
}

func (r *changesetApplyPreviewConnectionResolver) compute(ctx context.Context) (ee.RewirerMappings, error) {
	r.once.Do(func() {
		opts := r.opts
		opts.CampaignSpecID = r.campaignSpecID
		r.mappings, r.err = r.store.GetRewirerMappings(ctx, opts)
		if r.err != nil {
			return
		}
		r.err = r.mappings.Hydrate(ctx, r.store)
	})

	return r.mappings, r.err
}
