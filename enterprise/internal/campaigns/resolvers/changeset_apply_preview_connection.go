package resolvers

import (
	"context"
	"sync"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	ee "github.com/sourcegraph/sourcegraph/enterprise/internal/campaigns"
	"github.com/sourcegraph/sourcegraph/internal/campaigns"
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
	campaign *campaigns.Campaign
	err      error
}

func (r *changesetApplyPreviewConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	mappings, _, err := r.compute(ctx)
	if err != nil {
		return 0, err
	}
	return int32(len(mappings)), nil
}

func (r *changesetApplyPreviewConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	return graphqlutil.HasNextPage(false), nil
}

func (r *changesetApplyPreviewConnectionResolver) Nodes(ctx context.Context) ([]graphqlbackend.ChangesetApplyPreviewResolver, error) {
	mappings, campaign, err := r.compute(ctx)
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
			preloadedCampaign: campaign,
		})
	}

	return resolvers, nil
}

func (r *changesetApplyPreviewConnectionResolver) compute(ctx context.Context) (ee.RewirerMappings, *campaigns.Campaign, error) {
	r.once.Do(func() {
		opts := r.opts
		opts.CampaignSpecID = r.campaignSpecID

		svc := ee.NewService(r.store, nil)
		campaignSpec, err := r.store.GetCampaignSpec(ctx, ee.GetCampaignSpecOpts{ID: r.campaignSpecID})
		if err != nil {
			r.err = err
			return
		}
		// Dry-run reconcile the campaign with the new campaign spec.
		r.campaign, _, err = svc.ReconcileCampaign(ctx, campaignSpec)
		if err != nil {
			r.err = err
			return
		}

		opts.CampaignID = r.campaign.ID

		r.mappings, r.err = r.store.GetRewirerMappings(ctx, opts)
		if r.err != nil {
			return
		}
		r.err = r.mappings.Hydrate(ctx, r.store)
	})

	return r.mappings, r.campaign, r.err
}
